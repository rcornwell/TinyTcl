/*
 * TCL  Expect command processing.
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
	"net"
	"os"
	"os/exec"
	"time"

	pty "github.com/creack/pty"
	tcl "github.com/rcornwell/tinyTCL/tcl"
)

// expect ?-re ?-gl ?-ex ?-nocase string
// expect block
// expect_user
//  block is match action.
// timeout
// connected
// eof
// default   timeout|eof
// action abort
//
// Variables.
// expect_out#
// timeout
// spawn_id
// send_human {time .1 second }
//
// expect_continue
// connect ?-raw host ?port
// spawn prog args => procid
// send ?-null ?-break ?-s ?-h ?-i spawn_id string
// send_error ...
// send_log ... set to log_file.
// send_user ...
// disconnect/close -i spawn_id
// sleep seconds
// interact match action
// log_file ?-open ?-leaveopen ?-noappend file
// log_user 0|1
// match_max ?-d ?-i spawn_id ?size

const (
	ExpContinue = iota + tcl.RetExit + 1
	ExpEnd
)

type expectProcess struct {
	pty          *os.File      // Pty connection.
	rdr          *streamReader // Reader processes.
	matchData    matchBuffer   // current buffer being matched.
	matchPats    []*matchList  // List of current expect.
	connect      net.Conn      // Network connection.
	command      *exec.Cmd     // Current executing command, nil if network connection.
	state        *tnState      // Current telnet state.
	last         []byte        // Last characters received.
	matching     bool          // Matching input.
	tcl          *tcl.Tcl      // Pointer to interpreter we are running under.
	stdinTimeOut int           // Timeout for stdin.
	readTimeOut  int           // Timeout for remote input.
}

type expectData struct {
	processes  map[string]*expectProcess
	spawnCount int // next spawn ID.
}

// Register commands.
func Init(tcl *tcl.Tcl) {
	tcl.Register("connect", cmdConnect)
	tcl.Register("disconnect", cmdDisconnect)
	tcl.Register("expect", cmdExpect)
	tcl.Register("expect_continue", cmdExpectContinue)
	tcl.Register("interact", cmdInteract)
	tcl.Register("send", cmdSend)
	tcl.Register("sleep", cmdSleep)
	tcl.Register("spawn", cmdSpawn)
	tcl.Register("wait", cmdWait)
	tcl.SetVarValue("timeout", "-1")

	data := expectData{}
	tcl.Data["expect"] = &data
	data.processes = make(map[string]*expectProcess)
}

// Continue for expect.
func cmdExpectContinue(t *tcl.Tcl, _ []string) int {
	return t.SetResult(ExpContinue, "")
}

// Process expect command.
func cmdExpect(t *tcl.Tcl, args []string) int {
	spawnID := ""
	i := 1
outer:
	// Scan arguments, ?-i spawnId ?-- expect routine or block.
	for ; i < len(args); i++ {
		switch args[i] {
		case "--":
			break outer
		case "-i":
			i++
			if i >= len(args) {
				return t.SetResult(tcl.RetError, "-i missing argument")
			}
			spawnID = args[i]
		default:
			break outer
		}
	}

	// Check if spawnID is valid.
	if spawnID == "" {
		ok, id := t.GetVarValue("spawn_id")
		if ok != tcl.RetOk {
			return t.SetResult(tcl.RetError, "spawn_id variable not defined")
		}
		spawnID = id
	}

	expect, eok := t.Data["expect"].(*expectData)
	if !eok {
		panic("invalid data type expect extension")
	}

	proc, sok := expect.processes[spawnID]
	if !sok {
		return t.SetResult(tcl.RetError, "no process of name "+spawnID)
	}

	var patterns []string
	// Build match patterns.
	if (i + 1) == len(args) {
		//	fmt.Printf("match: %d %d\n", i, len(args))
		patterns = tcl.NewTCL().ParseArgs(args[i])
	} else {
		patterns = args[i:]
	}
	_, mlout := scanMatch(patterns, false)

	proc.matchPats = mlout
	proc.matching = true
	proc.matchData.matchBuffer = ""
	proc.matchData.Length = -1

	// Get timeout value.
	ok, timeout := t.GetVarValue("timeout")
	proc.readTimeOut = -1
	if ok == tcl.RetOk {
		fmt.Println(timeout)
		timeOut, _, ok := tcl.ConvertStringToNumber(timeout, 10, 0)
		if !ok {
			return t.SetResult(tcl.RetError, "timeout variable not defined.")
		}
		proc.readTimeOut = timeOut
	}
	proc.matching = true
	defer func() { proc.matching = false }()

	// Process any unprocessed input.
	if len(proc.last) != 0 {
		ret, _ := processRemote(proc, proc.last, nil)
		proc.last = []byte{}
		switch ret {
		case ExpEnd:
			return t.SetResult(tcl.RetOk, "")
		case tcl.RetError, tcl.RetBreak, tcl.RetReturn, tcl.RetContinue, tcl.RetOk:
			return t.SetResult(ret, "")
		}
	}

	fmt.Println("Expect")
	// Start reading input.
	proc.rdr.startReader(nil)
	defer proc.rdr.stopReader()
expect:
	for {
		ret := proc.rdr.wait()
		fmt.Printf("wait %d\n", ret)
		switch ret {
		case -1, ExpContinue:
			//		proc.rdr.startReader(nil)
		case ExpEnd, tcl.RetExit:
			break expect
		case tcl.RetError, tcl.RetBreak, tcl.RetReturn, tcl.RetContinue, tcl.RetOk:
			return t.SetResult(ret, "")
		}
	}
	return t.SetResult(tcl.RetOk, "")
}

// Talk interactively to remote command.
func cmdInteract(t *tcl.Tcl, args []string) int {
	// Process arguments.
	spawnID := ""
	i := 1
outer:
	for ; i < len(args); i++ {
		switch args[i] {
		case "--":
			break outer
		case "-i":
			i++
			if i >= len(args) {
				return t.SetResult(tcl.RetError, "-i missing argument")
			}
			spawnID = args[i]
		default:
			break outer
		}
	}

	// Figure out spawnID to talk to.
	if spawnID == "" {
		ok, id := t.GetVarValue("spawn_id")
		if ok != tcl.RetOk {
			return t.SetResult(tcl.RetError, "spawn_id variable not defined")
		}
		spawnID = id
	}

	expect, eok := t.Data["expect"].(*expectData)
	if !eok {
		panic("invalid data type expect extension")
	}

	proc, sok := expect.processes[spawnID]
	if !sok {
		return t.SetResult(tcl.RetError, "no process of name "+spawnID)
	}

	var patterns []string
	// Build match patterns.
	if (i + 1) == len(args) {
		patterns = tcl.NewTCL().ParseArgs(args[1])
	} else {
		patterns = args[1:]
	}

	// Set up how to match input/output.
	mlin, mlout := scanMatch(patterns, true)
	proc.matchPats = mlout
	proc.matching = true
	proc.matchData.matchBuffer = ""
	proc.matchData.Length = -1
	proc.stdinTimeOut = getTimeout(mlin)
	proc.readTimeOut = getTimeout(mlout)
	defer func() { proc.matching = false }()
	mbuf := matchBuffer{Length: -1, Max: 2000}

	if len(proc.last) != 0 {
		_, _ = processRemote(proc, proc.last, nil)
		proc.last = []byte{}
	}

	proc.rdr.startReader(os.Stdin)
	defer proc.rdr.stopReader()
	for {
		ret := proc.rdr.read(t, proc, mlin, &mbuf)
		fmt.Printf("Read %d\n", ret)
		if ret >= 0 {
			switch ret {
			case ExpEnd, tcl.RetExit:
				return t.SetResult(tcl.RetOk, "")
			case tcl.RetError, tcl.RetBreak, tcl.RetReturn, tcl.RetContinue:
				return t.SetResult(ret, "")
			case ExpContinue:
				continue
			}
			continue
		}

	}
}

// Send string to output.
func cmdSend(t *tcl.Tcl, args []string) int {
	spawnID := ""
	sendNull := false
	numNull := 0
	sendBreak := false
	i := 1
outer:
	for ; i < len(args); i++ {
		switch args[i] {
		case "--":
			break outer
		case "-i":
			i++
			if i >= len(args) {
				return t.SetResult(tcl.RetError, "-i missing argument")
			}
			spawnID = args[i]

		case "-null":
			sendNull = true
			i++
			if i >= len(args) {
				numNull = 1
				break outer
			}
			numNull, _, sendNull = tcl.ConvertStringToNumber(args[i], 10, 0)
			if !sendNull {
				numNull = 1
			}

		case "-break":
			sendBreak = true

		case "-h":

		case "-s":

		default:
			break outer
		}
	}

	var send []byte
	switch {
	case sendNull:
		send = make([]byte, numNull)
	case sendBreak:
		send = []byte{tnBRK}
	case i >= len(args):
		return t.SetResult(tcl.RetError, "send string")
	default:
		send = []byte(args[i])
	}

	if spawnID == "" {
		ok, id := t.GetVarValue("spawn_id")
		if ok != tcl.RetOk || id == "" {
			fmt.Println(send)
			return t.SetResult(tcl.RetOk, "")
		}
		spawnID = id
	}

	expect, eok := t.Data["expect"].(*expectData)
	if !eok {
		panic("invalid data type expect extension")
	}

	proc, sok := expect.processes[spawnID]
	if !sok {
		return t.SetResult(tcl.RetError, "no process of name "+spawnID)
	}

	err := proc.rdr.write(proc, send)
	if err != nil {
		return t.SetResult(tcl.RetError, err.Error())
	}
	return t.SetResult(tcl.RetOk, "")
}

// Connect to TCP host.
func cmdConnect(t *tcl.Tcl, args []string) int {
	if len(args) < 2 {
		return t.SetResult(tcl.RetError, "connect host ?port")
	}
	expect, ok := t.Data["expect"].(*expectData)
	if !ok {
		panic("invalid data type expect extension")
	}
	spawnID := "spawn" + tcl.ConvertNumberToString(expect.spawnCount, 10)
	t.SetVarValue("spawn_id", spawnID)
	expect.spawnCount++
	proc := expectProcess{tcl: t, last: []byte{}, matchData: matchBuffer{Length: -1, Max: 2000}}
	expect.processes[spawnID] = &proc
	host := args[1]
	port := "23"
	if len(args) > 2 {
		port = args[2]
	}

	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return t.SetResult(tcl.RetError, err.Error())
	}
	proc.connect = conn
	proc.rdr = newReader(&proc)
	proc.state = openTelnet(proc.connect)
	return t.SetResult(tcl.RetOk, "")
}

// Spawn a process on a pty.
func cmdSpawn(t *tcl.Tcl, args []string) int {
	// Check at least one argument.
	if len(args) < 2 {
		return t.SetResult(tcl.RetError, "spawn proc ?arg")
	}

	// Grab expect structure and allocate a new ID.
	expect, ok := t.Data["expect"].(*expectData)
	if !ok {
		panic("invalid data type expect extension")
	}
	spawnID := "spawn" + tcl.ConvertNumberToString(expect.spawnCount, 10)
	t.SetVarValue("spawn_id", spawnID)
	expect.spawnCount++
	proc := expectProcess{tcl: t, last: []byte{}, matchData: matchBuffer{Length: -1, Max: 2000}}
	expect.processes[spawnID] = &proc
	cmd := exec.Command(args[1], args[2:]...)
	proc.command = cmd
	if cmd == nil {
		return t.SetResult(tcl.RetError, "unable to start process")
	}
	p, err1 := pty.Start(cmd)
	proc.pty = p
	proc.rdr = newReader(&proc)

	if err1 != nil {
		// Call disconnect.
		_ = cmdDisconnect(t, []string{"disconnect"})
		return t.SetResult(tcl.RetError, err1.Error())
	}
	return t.SetResult(tcl.RetOk, tcl.ConvertNumberToString(cmd.Process.Pid, 10))
}

// Sleep for a number of seconds.
func cmdSleep(t *tcl.Tcl, args []string) int {
	// Validate arguments.
	if len(args) != 2 {
		return t.SetResult(tcl.RetError, "sleep seconds")
	}
	sleepTime, _, ok := tcl.ConvertStringToNumber(args[1], 10, 0)
	if !ok {
		return t.SetResult(tcl.RetError, "time not a number")
	}
	time.Sleep(time.Duration(sleepTime) * time.Second)
	return t.SetResult(tcl.RetOk, "")
}

// Wait for process to exit.
func cmdWait(t *tcl.Tcl, args []string) int {
	// Process arguments.
	spawnID := ""
	i := 1
outer:
	for ; i < len(args); i++ {
		switch args[i] {
		case "-i":
			i++
			if i >= len(args) {
				return t.SetResult(tcl.RetError, "-i missing argument")
			}
			spawnID = args[i]
		default:
			break outer
		}
	}

	// Find process to wait on.
	if spawnID == "" {
		ok, id := t.GetVarValue("spawn_id")
		if ok != tcl.RetOk {
			return t.SetResult(tcl.RetError, "spawn_id variable not defined")
		}
		spawnID = id
	}

	expect, eok := t.Data["expect"].(*expectData)
	if !eok {
		panic("invalid data type expect extension")
	}

	proc, sok := expect.processes[spawnID]
	if !sok {
		return t.SetResult(tcl.RetError, "no process of name "+spawnID)
	}

	// Close any open connections.
	if proc.rdr.close() {
		delete(expect.processes, spawnID)
		return t.SetResult(tcl.RetOk, "")
	}

	// Save the command pointer before deleting spawn structure.
	cmd := proc.command
	delete(expect.processes, spawnID)
	if cmd != nil {
		err := cmd.Wait()
		if err != nil {
			code, ok := err.(*exec.ExitError)
			if ok {
				return t.SetResult(tcl.RetOk, tcl.ConvertNumberToString(code.ExitCode(), 10))
			}
		}
	}
	return t.SetResult(tcl.RetOk, "")
}

// Disconnect or close a spawned/connected process.
// Can't use close since that is used by file extension.
func cmdDisconnect(t *tcl.Tcl, args []string) int {
	// Scan arguments.
	spawnID := ""
	i := 1
outer:
	for ; i < len(args); i++ {
		switch args[i] {
		case "-i":
			i++
			if i >= len(args) {
				return t.SetResult(tcl.RetError, "-i missing argument")
			}
			spawnID = args[i]
		default:
			break outer
		}
	}

	// Get process to close.
	if spawnID == "" {
		ok, id := t.GetVarValue("spawn_id")
		if ok != tcl.RetOk {
			return t.SetResult(tcl.RetError, "spawn_id variable not defined")
		}
		spawnID = id
	}

	expect, eok := t.Data["expect"].(*expectData)
	if !eok {
		panic("invalid data type expect extension")
	}

	proc, sok := expect.processes[spawnID]
	if !sok {
		return t.SetResult(tcl.RetError, "no process of name "+spawnID)
	}

	// Close open connections.
	if proc.rdr.close() {
		// For network we can just delete connection data.
		delete(expect.processes, spawnID)
	}

	return t.SetResult(tcl.RetOk, "")
}
