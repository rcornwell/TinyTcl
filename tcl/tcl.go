/*
 * TCL  basic TCL interpreter.
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

package tcl

import (
	"errors"
	"strings"
)

// Return codes from TCL builtins.
const (
	RetOk       = iota // All ok.
	RetError           // Error, message in results.
	RetReturn          // Return result in results.
	RetBreak           // Break statement.
	RetContinue        // Continue statement.
	RetExit            // Exit TCL system.
)

var (
	ErrExit  = errors.New("exit")
	ErrError = errors.New("error")
)

// Holds information about current running TCL session.
type Tcl struct {
	env    *tclEnv            // Variables.
	level  int                // Current nesting level.
	cmds   map[string]*tclCmd // Supported commands.
	result string             // Result from last command.
	Data   map[string]any     // Place for extensions to store data.
}

// Commands, function amd default arguments.
type tclCmd struct {
	fn   func(*Tcl, []string, []string) int
	proc bool
	arg  []string
}

// Holds data relative to variables.
type tclVar struct {
	value string
}

// Current running environment.
type tclEnv struct {
	vars   map[string]*tclVar // All excessible variables.
	local  map[string]bool    // Is this name local to procedure
	parent *tclEnv            // Parent nesting level.
	args   string             // Current arguments.
}

// Create new environment to execute TCl commands.
func NewTCL() *Tcl {
	tcl := &Tcl{}
	tcl.env = tcl.newEnv()
	tcl.cmds = make(map[string]*tclCmd)
	tcl.Data = make(map[string]any)
	tcl.tclInitCommands()
	return tcl
}

// Set results of last command.
func (tcl *Tcl) SetResult(err int, str string) int {
	tcl.result = str
	return err
}

// Get the results.
func (tcl *Tcl) GetResult() string {
	return tcl.result
}

// Evaluate a string, and return result code as string.
func (tcl *Tcl) EvalString(str string) error {
	ret := tcl.eval(str, parserOptions{})
	switch ret {
	case RetOk:
		return nil
	case RetExit:
		return ErrExit
	default:
		return ErrError
	}
}

// Evaluate a TCL expression.
func (tcl *Tcl) eval(str string, opts parserOptions) int {
	tcl.result = ""
	if str == "" {
		return RetOk
	}
	args := []string{}
	prevToken := tokEOL
	p := newParser(str, opts)

	for {
		if !p.getToken() {
			tcl.result = "error parsing: " + str
			return RetError
		}
		if p.token == tokEOF {
			break
		}
		val := p.GetString()

		switch p.token {
		case tokVar: // If variable, replace with value.
			ok, result := tcl.GetVarValue(val)
			if ok != RetOk {
				tcl.result = result
				return RetError
			}
			if len(result) == 0 {
				val = ""
			} else {
				val = result
			}

		case tokCmd: // Got command, try and execute it.
			err := tcl.eval(val, parserOptions{})
			if err != RetOk {
				if p.options.noEval {
					if err == RetBreak {
						tcl.result = strings.Join(args, " ")
						return RetOk
					}
					if err == RetContinue || err == RetReturn {
						val = tcl.result
						break
					}
				}
				return err
			}
			val = tcl.result

		case tokEscape: // Clean up any escape sequences in string.
			val, _ = UnEscape(val)

		case tokSpace: // Blank go grab next token.
			prevToken = p.token
			continue
		}

		// If we hit end of line, either create string of argument, or call function.
		if p.token == tokEOL {
			prevToken = p.token
			if opts.noEval {
				tcl.result = strings.Join(args, " ")
			} else {
				err := tcl.doCommand(args)
				if err != RetOk {
					return err
				}
			}
			args = []string{}
			continue
		}

		// If previous was blank or end of line, append to argument list.
		if prevToken == tokSpace || prevToken == tokEOL {
			args = append(args, val)
		} else {
			args[len(args)-1] += val
		}
		prevToken = p.token
	}
	return RetOk
}

// Scan a string and extract a list of arguments.
func (tcl *Tcl) ParseArgs(str string) []string {
	if str == "" {
		return []string{""}
	}
	res := []string{}
	p := newParser(str, parserOptions{noCommands: true, noEscapes: true, noVars: true, noEval: true})
	for {
		if !p.getToken() {
			return []string{}
		}
		switch p.token {
		case tokEOF:
			return res
		case tokString, tokVar, tokCmd, tokEscape:
			res = append(res, p.GetString())
		}
	}
}

// Execute a command.
func (tcl *Tcl) doCommand(args []string) int {
	// Empty commands just do nothing.
	if len(args) == 0 {
		return RetOk
	}
	tcl.result = ""
	cmd, ok := tcl.cmds[args[0]]
	if !ok {
		tcl.result = "unable to find command: " + args[0]
		return RetError
	}
	return cmd.fn(tcl, args, cmd.arg)
}
