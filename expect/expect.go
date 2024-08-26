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
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"

	pty "github.com/creack/pty"
	tcl "github.com/rcornwell/tinyTCL/tcl"
)

const (
	ExpContinue = iota + tcl.RetExit + 1
	ExpEnd
)

const (
	SendRemote = iota + 1
	SendUser
	SendLog
	SendError
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
	spawnCount int      // next spawn ID.
	matchMax   int      // Maximum number of characters for match buffer.
	logFile    *os.File // Logging file.
	logUser    bool     // Log output to user.
	logAll     bool     // Always log to user.
}

// Register commands.
func Init(t *tcl.Tcl) {
	t.Register("connect", cmdConnect)
	t.Register("disconnect", cmdDisconnect)
	t.Register("expect", cmdExpect)
	t.Register("expect_continue", cmdExpectContinue)
	t.Register("interact", cmdInteract)
	t.Register("log_file", cmdLogFile)
	t.Register("log_user", cmdLogUser)
	t.Register("match_max", cmdMatchMax)
	t.Register("send", func(t *tcl.Tcl, args []string) int {
		return cmdSend(t, args, SendRemote)
	})
	t.Register("send_error", func(t *tcl.Tcl, args []string) int {
		return cmdSend(t, args, SendError)
	})
	t.Register("send_log", func(t *tcl.Tcl, args []string) int {
		return cmdSend(t, args, SendLog)
	})
	t.Register("send_user", func(t *tcl.Tcl, args []string) int {
		return cmdSend(t, args, SendUser)
	})
	t.Register("sleep", cmdSleep)
	t.Register("spawn", cmdSpawn)
	t.Register("wait", cmdWait)
	t.SetVarValue("timeout", "-1")

	data := expectData{matchMax: 2000, logUser: true}
	t.Data["expect"] = &data
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
		patterns = t.ParseArgs(args[i])
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
		ret := processRemote(proc, proc.last, nil)
		proc.last = []byte{}
		switch ret {
		case ExpEnd:
			return t.SetResult(tcl.RetOk, "")
		case tcl.RetError, tcl.RetBreak, tcl.RetReturn, tcl.RetContinue, tcl.RetOk:
			return t.SetResult(ret, "")
		}
	}

	// Start reading input.
	proc.rdr.setLogging(expect.logFile, expect.logUser, expect.logAll)
	proc.rdr.startReader(nil)
	defer proc.rdr.stopReader()
expect:
	for {
		ret := proc.rdr.wait()
		switch ret {
		case -1, ExpContinue:
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
	mbuf := matchBuffer{Length: -1, Max: expect.matchMax}

	if len(proc.last) != 0 {
		_ = processRemote(proc, proc.last, nil)
		proc.last = []byte{}
	}

	proc.rdr.setLogging(expect.logFile, false, false)
	proc.rdr.startReader(os.Stdin)
	defer proc.rdr.stopReader()
	for {
		ret := proc.rdr.read(t, proc, mlin, &mbuf)
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
func cmdSend(t *tcl.Tcl, args []string, dest int) int {
	spawnID := ""
	sendNull := false
	numNull := 0
	sendBreak := false
	chunk := 0
	timeout := 0
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
			ret, val := t.GetVarValue("send_human")
			if ret == tcl.RetOk {
				times := scanSend(val)
				if len(times) > 0 {
					chunk = times[0] / 1000
				}
				if len(times) > 1 {
					timeout = times[1]
				}
			}

		case "-s":
			ret, val := t.GetVarValue("send_slow")
			if ret == tcl.RetOk {
				times := scanSend(val)
				if len(times) > 0 {
					chunk = times[0] / 1000
				}
				if len(times) > 1 {
					timeout = times[1]
				}
			}

		default:
			break outer
		}
	}

	var send string
	switch {
	case sendNull:
		send = strings.Repeat("\000", numNull)
	case sendBreak:
		send = string(tnBRK)
	case i >= len(args):
		return t.SetResult(tcl.RetError, "send string")
	default:
		send = args[i]
	}

	if spawnID == "" {
		ok, id := t.GetVarValue("spawn_id")
		if ok != tcl.RetOk || id == "" {
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

	chunks := chunkString(send, chunk)

	for _, sendString := range chunks {
		var err error
		switch dest {
		case SendRemote:
			err = proc.rdr.write(proc, []byte(sendString))
		case SendError:
			_, err = os.Stderr.Write([]byte(sendString))
		case SendLog:
			if expect.logFile != nil {
				_, err = expect.logFile.Write([]byte(sendString))
			}
		case SendUser:
			_, err = os.Stdout.Write([]byte(sendString))
		}
		if err != nil {
			return t.SetResult(tcl.RetError, err.Error())
		}
		if timeout != 0 {
			time.Sleep(time.Millisecond * time.Duration(timeout))
		}
	}
	return t.SetResult(tcl.RetOk, "")
}

// Scan variable and return times in milliseconds.
func scanSend(str string) []int {
	results := []int{}
	val := 0
	haveNum := false
	scale := 1000 // How much to scale number by.
	dot := false
	for _, d := range str {
		// Skip leading spaces.
		if unicode.IsSpace(d) {
			// On space if we have value,
			if haveNum {
				results = append(results, val*scale)
				val = 0
				haveNum = false
				scale = 1000
			}
			continue
		}

		// If we have . start scaling.
		if d == '.' {
			dot = true
			continue
		}

		// If not digit, exit.
		if d < '0' || d > '9' {
			break
		}
		val = (val * 10) + int(d-'0')
		haveNum = true
		if dot {
			scale /= 10
		}
	}

	// Make sure any trailing number is inserted.
	if haveNum {
		results = append(results, val*scale)
	}
	return results
}

// Chunk string into a series of character to send at a time.
func chunkString(str string, chunk int) []string {
	if len(str) == 0 {
		return []string{}
	}
	if chunk == 0 || chunk >= len(str) {
		return []string{str}
	}
	ret := []string{}
	length := 0
	start := 0
	for i := range str {
		if length == chunk {
			ret = append(ret, str[start:i])
			length = 0
			start = i
		}
		length++
	}
	ret = append(ret, str[start:])
	return ret
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
	proc := expectProcess{tcl: t, last: []byte{}, matchData: matchBuffer{Length: -1, Max: expect.matchMax}}
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

// Set log file.
func cmdLogFile(t *tcl.Tcl, args []string) int {
	name := ""
	mode := os.O_WRONLY | os.O_TRUNC | os.O_CREATE
	perm := 0o666
	all := false
	expect, eok := t.Data["expect"].(*expectData)
	if !eok {
		panic("invalid data type expect extension")
	}

	// Process arguments.
	i := 1
outer:
	for ; i < len(args); i++ {
		switch args[i] {
		case "-noappend":
			mode = os.O_WRONLY | os.O_APPEND | os.O_CREATE

		case "-a":
			all = true

		case "-info":
			res := ""
			if expect.logFile != nil {
				if expect.logAll {
					res += "-a "
				}
				res += expect.logFile.Name()
			}
			return t.SetResult(tcl.RetOk, res)

		default:
			file, err := os.OpenFile(args[i], mode, os.FileMode(perm))
			if err != nil {
				return t.SetResult(tcl.RetError, "unable to open file "+name+" "+err.Error())
			}
			expect.logFile = file
			expect.logAll = all
			break outer
		}
	}

	return t.SetResult(tcl.RetOk, "")
}

// Set output to user.
func cmdLogUser(t *tcl.Tcl, args []string) int {
	if len(args) != 2 {
		return t.SetResult(tcl.RetError, "log_user -info|0|1")
	}

	expect, eok := t.Data["expect"].(*expectData)
	if !eok {
		panic("invalid data type expect extension")
	}

	res := ""
	switch args[1] {
	case "-info":
		res = "0"
		if expect.logUser {
			res = "1"
		}
	case "0":
		expect.logUser = false
	case "1":
		expect.logUser = true
	default:
		return t.SetResult(tcl.RetError, "log_user -info|0|1")
	}

	return t.SetResult(tcl.RetOk, res)
}

// Set match Max number.
func cmdMatchMax(t *tcl.Tcl, args []string) int {
	// Scan arguments.
	global := false
	spawnID := ""
	i := 1
	max := -1
outer:
	for ; i < len(args); i++ {
		switch args[i] {
		case "-d":
			global = true
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

	if i < len(args) {
		m, _, ok := tcl.ConvertStringToNumber(args[i], 10, 0)
		if !ok || m < 0 {
			return t.SetResult(tcl.RetError, "match_max not numeric argument")
		}
		max = m
	}

	// Get process to close.
	if spawnID == "" {
		ok, id := t.GetVarValue("spawn_id")
		if ok == tcl.RetOk {
			spawnID = id
		}
	}

	expect, eok := t.Data["expect"].(*expectData)
	if !eok {
		panic("invalid data type expect extension")
	}

	if spawnID == "" || global {
		if global && max >= 0 {
			expect.matchMax = max
		}
		return t.SetResult(tcl.RetOk, tcl.ConvertNumberToString(expect.matchMax, 10))
	}

	proc, sok := expect.processes[spawnID]
	if !sok {
		return t.SetResult(tcl.RetError, "no process of name "+spawnID)
	}

	if max >= 0 {
		proc.matchData.Max = max
	}
	return t.SetResult(tcl.RetOk, tcl.ConvertNumberToString(proc.matchData.Max, 10))
}
