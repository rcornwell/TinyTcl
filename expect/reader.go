/*
 * TCL  Expect process reader.
 *
 * Copyright 2024, Richard Cornwell
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 *
 */

package expect

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/muesli/cancelreader"
	"github.com/rcornwell/tinyTCL/tcl"
)

type readData struct {
	data []byte
	err  error
}

type streamReader struct {
	wg            sync.WaitGroup
	rdr           cancelreader.CancelReader // Input for PTY and network connection.
	running       bool                      // Remote reader process running.
	done          bool                      // Stop remote reader.
	proc          *expectProcess            // Pointer to process structure.
	inFile        *os.File                  // Pointer to stdin.
	stdinRdr      cancelreader.CancelReader // Cancelable reader for stdin.
	stdinReadChan chan struct{}             // Ask for one character from stdin.
	stdinChan     chan readData             // Data returned from stdin.
	stdinTimer    *time.Timer               // Timer for stdin timeout.
	readTimer     *time.Timer               // Timer for remote timeout.
	exitChan      chan int                  // Signal matched for remote input.
	logFile       *os.File                  // File to log remote traffic too.
	logUser       bool                      // Log to user.
	logAll        bool                      // Always log to user.
}

// Create a new remote reader.
func newReader(proc *expectProcess) *streamReader {
	r := &streamReader{
		proc:      proc,
		stdinChan: make(chan readData, 1),
		exitChan:  make(chan int, 1),
	}
	return r
}

// Set logging on change.
func (r *streamReader) setLogging(logFile *os.File, logUser bool, logAll bool) {
	r.logFile = logFile
	r.logAll = logAll
	r.logUser = logUser
}

// Set up to start reading from remote and stdin if specified.
func (r *streamReader) startReader(in *os.File) {
	// Start timeout for remote connection.
	if r.proc.readTimeOut > 0 {
		r.readTimer = time.NewTimer(time.Second * time.Duration(r.proc.readTimeOut))
	} else {
		r.readTimer = time.NewTimer(time.Second)
		r.readTimer.Stop()
	}

	r.done = false
	// If remote reader not running, start it.
	if !r.running {
		if r.proc.pty != nil {
			r.rdr, _ = cancelreader.NewReader(r.proc.pty)
		} else {
			r.rdr, _ = cancelreader.NewReader(r.proc.connect)
		}
		go r.outReader()
	}

	// If we have input file, start reader on it.
	r.inFile = in
	if in != nil {
		if r.proc.stdinTimeOut > 0 {
			r.stdinTimer = time.NewTimer(time.Second * time.Duration(r.proc.stdinTimeOut))
		} else {
			r.stdinTimer = time.NewTimer(time.Second)
			r.stdinTimer.Stop()
		}
		r.stdinReadChan = make(chan struct{}, 1)
		r.wg.Add(1)
		r.stdinReadChan <- struct{}{}
		rdr, err := cancelreader.NewReader(in)
		if err == nil {
			r.stdinRdr = rdr
			go r.reader()
		}
	}
}

// Close done reader.
func (r *streamReader) stopReader() {
	r.done = true
	r.rdr.Cancel()
	if !r.readTimer.Stop() {
		<-r.readTimer.C
	}
	if r.inFile == nil {
		return
	}
	close(r.stdinReadChan)
	r.stdinRdr.Cancel()
	r.stdinTimer.Stop()
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		select {
		case <-r.stdinChan:
		default:
		}
		return
	case <-time.After(time.Second):
		return
	}
}

// Close remote connections.
func (r *streamReader) close() bool {
	r.done = true
	r.rdr.Close()
	if r.proc.pty != nil {
		r.proc.pty.Close()
	}
	if r.proc.connect != nil {
		r.proc.connect.Close()
		return true
	}
	return false
}

// Read a single character from standard in, or remote process.
func (r *streamReader) read(t *tcl.Tcl, proc *expectProcess, mlin []*matchList, mbuf *matchBuffer) int {
	for {
		select {
		// Wait for character from stdin, if found process it and request new one.
		case data := <-r.stdinChan:
			appendMatch(mlin, mbuf, data.data)
			ret, m := match(t, mlin, mbuf)
			switch ret {
			case -1:
			case tcl.RetOk:
			default:
				return ret
			}

			// If we did not match anything send data to remote.
			if !m {
				err := r.write(proc, data.data)
				if err != nil {
					return t.SetResult(tcl.RetError, "write: "+err.Error())
				}
			}

			r.stdinReadChan <- struct{}{}
			continue

		// Handle timeout.
		case <-r.stdinTimer.C:
			return matchSpecial(proc.tcl, mlin, "timeout")

		case <-r.readTimer.C:
			return matchSpecial(proc.tcl, proc.matchPats, "timeout")

		// Remote input matched something.
		case ret := <-r.exitChan:
			return ret
		}
	}
}

// Wait until there is a match on the remote side.
func (r *streamReader) wait() int {
	select {
	case ret := <-r.exitChan:
		return ret

	case <-r.readTimer.C:
		return matchSpecial(r.proc.tcl, r.proc.matchPats, "timeout")
	}
}

// Write string to output.
func (r *streamReader) write(proc *expectProcess, output []byte) error {
	var err error
	if proc.pty != nil {
		_, err = proc.pty.Write(output)
	}
	if proc.connect != nil {
		err = proc.state.sendTelnet(output)
	}
	return err
}

// Read from stdin, one character at a time, with ability to cancel input.
func (r *streamReader) reader() {
	r.done = false
	input := make([]byte, 1)
	defer r.wg.Done()
	for {
		_, ok := <-r.stdinReadChan
		if !ok {
			break
		}

		n, err := r.stdinRdr.Read(input)
		if err != nil {
			r.stdinChan <- readData{err: err}
			break
		}
		if n == 0 {
			continue
		}
		r.stdinChan <- readData{data: input[:n], err: nil}
	}
}

// Read input from remote host or pty. Process each input.
func (r *streamReader) outReader() {
	defer func() { r.running = false }()
	var n int
	var err error
	r.done = false
	r.running = true
	for !r.done {
		input := make([]byte, 1024)
		// Get data. Any error is considered end of file.
		n, err = r.rdr.Read(input)
		if err != nil {
			err = io.EOF
		}

		// If network connection, process the characters.
		if r.proc.connect != nil {
			input = r.proc.state.receiveTelnet(input, n)
			n = len(input)
		}

		// If done, Read was canceled, just exit.
		if r.done {
			break
		}

		if r.logFile != nil {
			_, _ = r.logFile.Write(input[:n])
		}

		if r.logUser || r.logAll {
			_, _ = os.Stdout.Write(input[:n])
		}

		if n == 0 && err == nil {
			continue
		}

		ret := processRemote(r.proc, input[:n], err)
		if ret >= 0 {
			r.exitChan <- ret
			break
		}
	}
}
