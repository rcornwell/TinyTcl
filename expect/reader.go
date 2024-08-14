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
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/rcornwell/tinyTCL/tcl"
)

type readData struct {
	data []byte
	err  error
}

type streamReader struct {
	wg        sync.WaitGroup
	rdr       io.ReadCloser
	running   bool
	proc      *expectProcess
	readChan  chan struct{}
	writeChan chan readData
	exitChan  chan int
	inFile    *os.File
	outProc   func(*expectProcess, []byte, error) (int, bool)
	done      bool
}

// func newReader(in *os.File, out io.Reader, fn func([]byte, error) bool) *streamReader {
func newReader(proc *expectProcess, fn func(*expectProcess, []byte, error) (int, bool)) *streamReader {
	r := &streamReader{
		proc:      proc,
		rdr:       io.ReadCloser(proc.pty),
		outProc:   fn,
		writeChan: make(chan readData, 1),
		exitChan:  make(chan int, 1),
	}
	return r
}

func (r *streamReader) startReader(in *os.File) {
	r.inFile = in
	if in != nil {
		r.readChan = make(chan struct{}, 1)
		r.wg.Add(1)
		r.readChan <- struct{}{}
		go r.reader()
	}
	// if !r.running {
	// 	go r.outReader()
	// }
}

func (r *streamReader) stopReader() {
	r.done = true
	if r.inFile == nil {
		return
	}
	close(r.readChan)

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		select {
		case <-r.writeChan:
		default:
		}
		return
	case <-time.After(time.Second):
		fmt.Println("Reader timeout")
		return
	}
}

func (r *streamReader) close() {
	r.done = true
	if r.proc.pty != nil {
		r.rdr.Close()
	}
	if r.proc.connect != nil {
		r.proc.connect.Close()
	}
}

func (r *streamReader) read(t *tcl.Tcl, proc *expectProcess, mlin []*matchList, mbuf *matchBuffer) int {
	if !r.running {
		go r.outReader()
	}
	for {
		select {
		case data := <-r.writeChan:
			fmt.Println("stdin read: " + string(data.data))
			appendMatch(mlin, mbuf, data.data)
			ret, m := match(t, mlin, mbuf)
			switch ret {
			case -1:
			case tcl.RetOk:
			default:
				return ret
			}
			if !m {
				err := r.write(proc, data.data)
				if err != nil {
					return t.SetResult(tcl.RetError, "write: "+err.Error())
				}
			}
			r.readChan <- struct{}{}
			continue

		case ret := <-r.exitChan:
			return ret
		}
	}
}

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

func (r *streamReader) wait() int {
	if !r.running {
		go r.outReader()
	}
	return <-r.exitChan
}

func (r *streamReader) reader() {
	r.done = false
	input := make([]byte, 1)
	defer r.wg.Done()
	for {
		_, ok := <-r.readChan
		if !ok {
			break
		}
		r.inFile.SetReadDeadline(time.Now().Add(time.Second))
		n, err := r.inFile.Read(input)
		if err != nil {
			r.writeChan <- readData{err: err}
			break
		}
		if n == 0 {
			continue
		}
		fmt.Printf("stdin %02x '%s'\n", input[0], string(input))
		r.writeChan <- readData{data: input[:n], err: nil}
	}
	fmt.Println("input done")
}

func (r *streamReader) outReader() {
	defer func() { r.running = false }()
	var n int
	var err error
	r.done = false
	r.running = true
	fmt.Println("Reader started")
	for !r.done {
		input := make([]byte, 1024)
		if r.proc.pty != nil {
			r.proc.pty.SetReadDeadline(time.Now().Add(time.Second))
			n, err = r.rdr.Read(input)
		} else {
			n, err = r.proc.connect.Read(input)
			input = r.proc.state.receiveTelnet(input, n)
			n = len(input)
		}
		if err != nil {
			r.exitChan <- ExpEnd
			break
		}
		fmt.Printf("outReader: '%s' %d\n", string(input), n)
		if n == 0 {
			continue
		}

		ret, _ := r.outProc(r.proc, input[:n], err)
		if ret >= 0 {
			r.exitChan <- ret
			break
		}
	}
	fmt.Println("Reader exit")
}
