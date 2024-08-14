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
	pty       *os.File      // Pty connection.
	rdr       *streamReader // Reader processes.
	matchData matchBuffer   // current buffer being matched.
	matchPats []*matchList  // List of current expect.
	connect   net.Conn      // Network connection.
	command   *exec.Cmd     // Current executing command, nil if network connection.
	state     *tnState      // Current telnet state.
	last      []byte        // Last characters received.
	matching  bool          // Matching input.
	tcl       *tcl.Tcl      // Pointer to interpreter we are running under.
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

	data := expectData{}
	tcl.Data["expect"] = &data
	data.processes = make(map[string]*expectProcess)
}

func cmdExpectContinue(t *tcl.Tcl, _ []string) int {
	return t.SetResult(ExpContinue, "")
}

func cmdExpect(t *tcl.Tcl, args []string) int {
	spawnID := ""
	// sendNull := false
	//	sendBreak := false
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
	defer func() { proc.matching = false }()

	if len(proc.last) != 0 {
		ret, _ := process(proc, proc.last, nil)
		//	fmt.Println("Handle last")
		proc.last = []byte{}
		switch ret {
		case ExpEnd:
			return t.SetResult(tcl.RetOk, "")
		case tcl.RetError, tcl.RetBreak, tcl.RetReturn, tcl.RetContinue, tcl.RetOk:
			return t.SetResult(ret, "")
		}
	}

	proc.rdr.startReader(nil)
	defer proc.rdr.stopReader()
expect:
	for {
		ret := proc.rdr.wait()
		//	fmt.Printf("wait %d\n", ret)
		switch ret {
		case -1, ExpContinue:
			//		proc.rdr.startReader(nil)
		case ExpEnd:
			break expect
		case tcl.RetError, tcl.RetBreak, tcl.RetReturn, tcl.RetContinue, tcl.RetOk:
			return t.SetResult(ret, "")
		}
	}
	return t.SetResult(tcl.RetOk, "")
}

func cmdSend(t *tcl.Tcl, args []string) int {
	spawnID := ""
	// sendNull := false
	//	sendBreak := false
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
		//	sendNull = true
		case "-break":
		//	sendBreak = true
		case "-h":
		case "-s":
		default:
			break outer
		}
	}

	if spawnID == "" {
		ok, id := t.GetVarValue("spawn_id")
		if ok != tcl.RetOk {
			return t.SetResult(tcl.RetError, "spawn_id variable not defined")
		}
		spawnID = id
	}

	if i >= len(args) {
		return t.SetResult(tcl.RetError, "send string")
	}
	expect, eok := t.Data["expect"].(*expectData)
	if !eok {
		panic("invalid data type expect extension")
	}

	proc, sok := expect.processes[spawnID]
	if !sok {
		return t.SetResult(tcl.RetError, "no process of name "+spawnID)
	}

	//	fmt.Println("send " + args[i])
	err := proc.rdr.write(proc, []byte(args[i]))
	if err != nil {
		return t.SetResult(tcl.RetError, err.Error())
	}
	return t.SetResult(tcl.RetOk, "")
}

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
	proc.rdr = newReader(&proc, process)
	proc.state = openTelnet(proc.connect)
	return t.SetResult(tcl.RetOk, "")
}

func cmdSleep(t *tcl.Tcl, args []string) int {
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

func cmdSpawn(t *tcl.Tcl, args []string) int {
	if len(args) < 2 {
		return t.SetResult(tcl.RetError, "spawn proc ?arg")
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
	cmd := exec.Command(args[1], args[2:]...)
	proc.command = cmd
	if cmd == nil {
		return t.SetResult(tcl.RetError, "unable to start process")
	}
	p, err1 := pty.Start(cmd)
	proc.pty = p
	proc.rdr = newReader(&proc, process)

	if err1 != nil {
		// Call disconnect.
		_ = cmdDisconnect(t, []string{"disconnect"})
		return t.SetResult(tcl.RetError, err1.Error())
	}
	return t.SetResult(tcl.RetOk, tcl.ConvertNumberToString(cmd.Process.Pid, 10))
}

func cmdInteract(t *tcl.Tcl, args []string) int {
	ok, spawnID := t.GetVarValue("spawn_id")
	if ok != tcl.RetOk {
		return t.SetResult(tcl.RetError, "spawn_id variable not defined")
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
	if len(args) == 2 {
		patterns = tcl.NewTCL().ParseArgs(args[1])
	} else {
		patterns = args[1:]
	}
	mlin, mlout := scanMatch(patterns, true)
	// mlin := []matchList{}
	// mlout := []matchList{}

	// exact := false
	// //	glob := false
	// echo := false
	// nobuffer := false
	// nocase := false
	// output := false
	// body := false
	// cmd := false
	// for _, v := range patterns {
	// 	if !exact && !body {
	// 		switch v {
	// 		case "-o":
	// 			output = true
	// 			continue

	// 		case "-ex":
	// 			exact = true
	// 			//			glob = false
	// 			continue

	// 		case "-gl":
	// 			exact = false
	// 			//			glob = true
	// 			continue

	// 		case "-echo":
	// 			echo = true
	// 			continue

	// 		case "-nobuffer":
	// 			nobuffer = true
	// 			continue

	// 		case "-nocase":
	// 			nocase = true
	// 			continue

	// 		default:
	// 		}
	// 	}
	// 	if body {
	// 		if output {
	// 			mlout[len(mlout)-1].body = v
	// 		} else {
	// 			mlin[len(mlin)-1].body = v
	// 		}
	// 		body = false
	// 		continue
	// 	}
	// 	if !exact {
	// 		switch v {
	// 		case "timeout", "eof":
	// 			cmd = true
	// 		}
	// 	}
	// 	m := matchList{
	// 		str:      v,
	// 		mpos:     -1,
	// 		bpos:     0,
	// 		exact:    exact,
	// 		nocase:   nocase,
	// 		echo:     echo,
	// 		cmd:      cmd,
	// 		nobuffer: nobuffer,
	// 	}
	// 	nocase = false
	// 	exact = false
	// 	echo = false
	// 	cmd = false
	// 	body = true
	// 	if output {
	// 		mlout = append(mlout, m)
	// 	} else {
	// 		mlin = append(mlin, m)
	// 	}
	// }

	proc.matchPats = mlout
	proc.matching = true
	proc.matchData.matchBuffer = ""
	proc.matchData.Length = -1
	defer func() { proc.matching = false }()
	mbuf := matchBuffer{Length: -1, Max: 2000}

	if len(proc.last) != 0 {
		_, _ = process(proc, proc.last, nil)
		proc.last = []byte{}
	}

	proc.rdr.startReader(os.Stdin)
	defer proc.rdr.stopReader()
	for {
		ret := proc.rdr.read(t, proc, mlin, &mbuf)
		if ret >= 0 {
			switch ret {
			case ExpEnd:
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

func cmdWait(t *tcl.Tcl, args []string) int {
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

	if proc.rdr != nil {
		proc.rdr.close()
		proc.rdr = nil
		//	fmt.Println("Reader closed")
	}
	if proc.pty != nil {
		proc.pty.Close()
		proc.pty = nil
		//		fmt.Println("pty closed")
	}
	if proc.connect != nil {
		proc.connect.Close()
		proc.connect = nil
		// fmt.Println("connect close")
	}

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

func cmdDisconnect(t *tcl.Tcl, args []string) int {
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

	if proc.rdr != nil {
		proc.rdr.close()
		proc.rdr = nil
		//	fmt.Println("Reader closed")
	}
	if proc.pty != nil {
		proc.pty.Close()
		proc.pty = nil
		//	fmt.Println("pty closed")
	}
	if proc.connect != nil {
		proc.connect.Close()
		delete(expect.processes, spawnID)
		//	fmt.Println("connect close")
	}

	return t.SetResult(tcl.RetOk, "")
}
