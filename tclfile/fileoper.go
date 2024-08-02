/*
 * TCL  commands for operating on files.
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

package tclfile

import (
	"io"
	"os"

	tcl "github.com/rcornwell/tinyTCL/tcl"
)

type tclFileData struct {
	channels map[string]*os.File // Pointer to open file names.
	eof      map[string]bool     // Has file hit EOF.
}

// Register commands.
func FileInit(tcl *tcl.Tcl) {
	tcl.Register("file", []string{}, false, cmdFile)
	tcl.Register("eof", []string{}, false, cmdEOF)
	tcl.Register("open", []string{}, false, cmdOpen)
	tcl.Register("close", []string{}, false, cmdClose)
	tcl.Register("gets", []string{}, false, cmdGets)
	tcl.Register("read", []string{}, false, cmdRead)
	tcl.Register("puts", []string{}, false, cmdPuts)
	tcl.Register("seek", []string{}, false, cmdSeek)
	tcl.Register("tell", []string{}, false, cmdSeek)
	tcl.Register("flush", []string{}, false, cmdFlush)
	data := tclFileData{}
	data.channels = make(map[string]*os.File)
	data.eof = make(map[string]bool)
	data.channels["stdin"] = os.Stdin
	data.eof["stdin"] = false
	data.channels["stdout"] = os.Stdout
	data.eof["stdout"] = false
	data.channels["stderr"] = os.Stderr
	data.eof["stderr"] = false
	tcl.Data["file"] = &data
}

// Open a file, return channel identifier.
func cmdOpen(t *tcl.Tcl, args []string, _ []string) int {
	name := ""
	access := "r"
	perms := "0666"
	switch len(args) {
	default:
		fallthrough
	case 0, 1:
		return t.SetResult(tcl.RetError, "open name ?access ?permissions")
	case 2:
		name = args[1]
	case 3:
		name = args[1]
		access = args[2]
	case 4:
		name = args[1]
		access = args[2]
		perms = args[3]
	}

	mode, ok := openModes[access]
	if !ok {
		return t.SetResult(tcl.RetError, "invalid access mode "+access)
	}

	perm, _, pok := tcl.ConvertStringToNumber(perms, 10, 0)
	if !pok {
		return t.SetResult(tcl.RetError, "invalid permissions "+perms)
	}

	file, err := os.OpenFile(name, mode, os.FileMode(perm))
	if err != nil {
		return t.SetResult(tcl.RetError, "unable to open file "+name+" "+err.Error())
	}

	files, ok := t.Data["file"].(*tclFileData)
	if !ok {
		panic("invalid data type file extension")
	}

	channel := "file" + tcl.ConvertNumberToString(int(file.Fd()), 10)
	files.channels[channel] = file
	files.eof[channel] = false
	return t.SetResult(tcl.RetOk, channel)
}

// Close a file based on channel idenifier.
func cmdClose(t *tcl.Tcl, args []string, _ []string) int {
	if len(args) < 2 || len(args) > 3 {
		return t.SetResult(tcl.RetError, "close channel")
	}

	files, ok := t.Data["file"].(*tclFileData)
	if !ok {
		panic("invalid data type file extension")
	}

	file, ok := files.channels[args[1]]
	if !ok {
		return t.SetResult(tcl.RetError, "file "+args[1]+" not opened")
	}

	err := file.Close()
	if err != nil {
		return t.SetResult(tcl.RetError, "unable to close file "+args[1]+" "+err.Error())
	}

	delete(files.channels, args[1])
	delete(files.eof, args[1])

	return t.SetResult(tcl.RetOk, "")
}

// Return if channel is at EOF.
func cmdEOF(t *tcl.Tcl, args []string, _ []string) int {
	if len(args) != 2 {
		return t.SetResult(tcl.RetError, "eof channel")
	}

	files, ok := t.Data["file"].(*tclFileData)
	if !ok {
		panic("invalid data type file extension")
	}

	eof, ok := files.eof[args[1]]
	if !ok {
		return t.SetResult(tcl.RetError, "file "+args[1]+" not opened")
	}
	if eof {
		return t.SetResult(tcl.RetOk, "1")
	}
	return t.SetResult(tcl.RetOk, "0")
}

// Return if channel is at EOF.
func cmdRead(t *tcl.Tcl, args []string, _ []string) int {
	if len(args) < 2 {
		return t.SetResult(tcl.RetError, "read ?-nonewline channel numchars")
	}

	files, ok := t.Data["file"].(*tclFileData)
	if !ok {
		panic("invalid data type file extension")
	}

	nonewline := false
	i := 1
	if args[i] == "-nonewline" {
		nonewline = true
		i++
	}

	if len(args) < i {
		return t.SetResult(tcl.RetError, "no channel given")
	}

	file, ok := files.channels[args[i]]
	if !ok {
		return t.SetResult(tcl.RetError, "file "+args[i]+" not opened")
	}

	bytes := 0
	info, err := file.Stat()
	if err != nil {
		return t.SetResult(tcl.RetError, "read error "+err.Error())
	}
	size := int(info.Size())
	if len(args) <= (i + 1) {
		pos, serr := file.Seek(0, 1)
		if serr != nil {
			return t.SetResult(tcl.RetError, "read error "+serr.Error())
		}
		bytes = size - int(pos)
	} else {
		bytes, _, ok = tcl.ConvertStringToNumber(args[i+1], 10, 0)
		if !ok {
			return t.SetResult(tcl.RetError, "can't convert number of bytes to integer")
		}
	}

	buffer := make([]byte, bytes)
	n, rerr := file.Read(buffer)
	if rerr != nil {
		return t.SetResult(tcl.RetError, "read error "+rerr.Error())
	}
	if n == 0 {
		files.eof[args[i]] = true
		return t.SetResult(tcl.RetOk, "")
	}

	if nonewline && buffer[n-1] == '\n' {
		n--
	}
	return t.SetResult(tcl.RetOk, string(buffer[:n]))
}

// Get a string from a channel.
func cmdGets(t *tcl.Tcl, args []string, _ []string) int {
	if len(args) < 2 || len(args) > 3 {
		return t.SetResult(tcl.RetError, "gets channel ?varname")
	}

	files, ok := t.Data["file"].(*tclFileData)
	if !ok {
		panic("invalid data type file extension")
	}

	if len(args) < 1 {
		return t.SetResult(tcl.RetError, "no channel given")
	}

	file, ok := files.channels[args[1]]
	if !ok {
		return t.SetResult(tcl.RetError, "file "+args[1]+" not opened")
	}

	buffer := ""
	input := make([]byte, 1)
	for {
		n, rerr := file.Read(input)
		if rerr != nil {
			return t.SetResult(tcl.RetError, "read error "+rerr.Error())
		}
		if n == 0 {
			files.eof[args[1]] = true
			return t.SetResult(tcl.RetOk, "")
		}
		if input[0] == '\n' {
			break
		}
		buffer += string(input[0])
	}

	if len(args) < 3 {
		return t.SetResult(tcl.RetOk, buffer)
	}
	t.SetVarValue(args[2], buffer)
	return t.SetResult(tcl.RetOk, tcl.ConvertNumberToString(len(buffer), 10))
}

// Write a string to a channel.
func cmdPuts(t *tcl.Tcl, args []string, _ []string) int {
	if len(args) < 2 {
		return t.SetResult(tcl.RetError, "puts ?-nonewline ?file text")
	}

	files, ok := t.Data["file"].(*tclFileData)
	if !ok {
		panic("invalid data type file extension")
	}

	nonewline := false
	file := files.channels["stdout"]
	i := 1
	if args[i] == "-nonewline" {
		nonewline = true
		i++
	}

	if len(args) > i {
		ok := false
		file, ok = files.channels[args[i]]
		if !ok {
			return t.SetResult(tcl.RetError, "file "+args[i]+" not opened")
		}
		i++
	}

	text := args[i]
	if !nonewline {
		text += "\n"
	}

	_, err := file.WriteString(text)
	if err != nil {
		return t.SetResult(tcl.RetError, err.Error())
	}
	return t.SetResult(tcl.RetOk, "")
}

// Seek or Tell command.
func cmdSeek(t *tcl.Tcl, args []string, _ []string) int {
	// tell channel ->  position
	// seek channel offset ?origin
	files, ok := t.Data["file"].(*tclFileData)
	if !ok {
		panic("invalid data type file extension")
	}

	if len(args) < 1 {
		return t.SetResult(tcl.RetError, "no channel given")
	}

	file, ok := files.channels[args[1]]
	if !ok {
		return t.SetResult(tcl.RetError, "file "+args[1]+" not opened")
	}
	origin := io.SeekStart
	offset := 0
	if args[0] == "seek" {
		if len(args) > 4 {
			return t.SetResult(tcl.RetError, "seek channel offset ?origin")
		}
		if len(args) >= 3 {
			o, _, ok := tcl.ConvertStringToNumber(args[2], 10, 0)
			if !ok {
				return t.SetResult(tcl.RetError, "offset not valid number")
			}
			offset = o
		}
		if len(args) == 4 {
			switch args[3] {
			case "start":
				origin = io.SeekStart
			case "current":
				origin = io.SeekCurrent
			case "end":
				origin = io.SeekEnd
			default:
				return t.SetResult(tcl.RetError, "invalid origin")
			}
		}
	}
	position, err := file.Seek(int64(offset), origin)
	if err != nil {
		return t.SetResult(tcl.RetError, err.Error())
	}
	if args[0] == "seek" {
		return t.SetResult(tcl.RetOk, "")
	}
	return t.SetResult(tcl.RetOk, tcl.ConvertNumberToString(int(position), 10))
}

// Flush any pending output for a channel.
func cmdFlush(t *tcl.Tcl, args []string, _ []string) int {
	if len(args) != 2 {
		return t.SetResult(tcl.RetError, "flush channel")
	}

	files, ok := t.Data["file"].(*tclFileData)
	if !ok {
		panic("invalid data type file extension")
	}

	err := files.channels[args[1]].Sync()
	if err != nil {
		return t.SetResult(tcl.RetError, err.Error())
	}
	return t.SetResult(tcl.RetOk, "")
}
