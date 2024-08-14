/*
 * TCL example interactive/script runner.
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

package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/peterh/liner"
	//expect "github.com/rcornwell/tinyTCL/expect"
	tcl "github.com/rcornwell/tinyTCL/tcl"
	file "github.com/rcornwell/tinyTCL/tclfile"
)

func main() {
	// Create new TCL environment.
	tinyTcl := tcl.NewTCL()
	tinyTcl.SetVarValue("argv0", os.Args[0])
	tinyTcl.SetVarValue("argc", "0")
	tinyTcl.SetVarValue("argv", "")

	// Add in file commands.
	file.Init(tinyTcl)

	// Add in expect commands.
	//expect.Init(tinyTcl)

	// If any arguments given, try to open the first one as a TCL file.
	if len(os.Args) > 2 {
		text, err := os.ReadFile(os.Args[1])
		if err != nil {
			panic(err)
		}
		tinyTcl.SetVarValue("argv0", os.Args[1])
		if len(os.Args) > 3 {
			tinyTcl.SetVarValue("argv", strings.Join(os.Args[2:], " "))
			tinyTcl.SetVarValue("argc", tcl.ConvertNumberToString(len(os.Args[3:]), 10))
		}
		err = tinyTcl.EvalString(string(text))
		if errors.Is(err, tcl.ErrError) {
			fmt.Println("Error: " + tinyTcl.GetResult())
		}
		os.Exit(0)
	}

	Line := liner.NewLiner()
	defer Line.Close()
	Line.SetCtrlCAborts(false)
	Line.SetMultiLineMode(true)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt)
	go func() {
		<-done
		Line.Close()
		fmt.Println("^C abort")
		os.Exit(0)
	}()

outer:
	for {
		multi := true
		command := ""
		for multi {
			line := ""
			var err error
			if command == "" {
				line, err = Line.Prompt("tcl> ")
			} else {
				line, err = Line.Prompt("tcl# ")
			}
			if err != nil {
				if errors.Is(err, liner.ErrPromptAborted) {
					fmt.Println("^C")
				} else {
					fmt.Println(err.Error())
				}
				break outer
			}
			if line == "" {
				continue
			}
			if line[len(line)-1:] == "\\" {
				command += line[:len(line)-1] + "\n"
			} else {
				command += line
				multi = false
			}
		}

		Line.AppendHistory(command)
		err := tinyTcl.EvalString(command)
		if err != nil {
			if errors.Is(err, tcl.ErrExit) {
				break
			} else {
				fmt.Println("Error: " + tinyTcl.GetResult())
			}
		} else if tinyTcl.GetResult() != "" {
			fmt.Println("=> " + tinyTcl.GetResult())
		}
	}
}
