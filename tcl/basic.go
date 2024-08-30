/*
 * TCL  basic TCL Commands
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
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Register commands.
func (tcl *Tcl) tclInitCommands() {
	tcl.Register("append", cmdAppend)
	tcl.Register("break", func(_ *Tcl, _ []string) int { return RetBreak })
	tcl.Register("catch", cmdCatch)
	tcl.Register("concat", cmdConcat)
	tcl.Register("continue", func(_ *Tcl, _ []string) int { return RetContinue })
	tcl.Register("decr", cmdDecr)
	tcl.Register("eq", cmdEqual)
	tcl.Register("error", cmdError)
	tcl.Register("eval", cmdEval)
	tcl.Register("exit", cmdExit)
	tcl.Register("expr", cmdMath)
	tcl.Register("for", cmdFor)
	tcl.Register("foreach", cmdForEach)
	tcl.Register("global", cmdGlobal)
	tcl.Register("if", cmdIf)
	tcl.Register("info", cmdInfo)
	tcl.Register("incr", cmdIncr)
	tcl.Register("join", cmdJoin)
	tcl.Register("lappend", cmdLAppend)
	tcl.Register("lindex", cmdLIndex)
	tcl.Register("linsert", cmdLInsert)
	tcl.Register("list", cmdList)
	tcl.Register("llength", cmdLLength)
	tcl.Register("lrange", cmdLRange)
	tcl.Register("lreplace", cmdLReplace)
	tcl.Register("lsearch", cmdLSearch)
	tcl.Register("lset", cmdLSet)
	tcl.Register("lsort", cmdLSort)
	tcl.Register("ne", cmdNotEqual)
	tcl.Register("proc", cmdProc)
	tcl.Register("puts", cmdPuts)
	tcl.Register("rename", cmdRename)
	tcl.Register("return", cmdReturn)
	tcl.Register("set", cmdSet)
	tcl.Register("split", cmdSplit)
	tcl.Register("string", cmdString)
	tcl.Register("subst", cmdSubst)
	tcl.Register("switch", cmdSwitch)
	tcl.Register("uplevel", cmdUpLevel)
	tcl.Register("upvar", cmdUpVar)
	tcl.Register("unset", cmdUnSet)
	tcl.Register("variable", cmdVariable)
	tcl.Register("while", cmdWhile)
}

// Register a command. Arg is passed to function when called.
func (tcl *Tcl) Register(name string, fn func(*Tcl, []string) int) {
	tcl.cmds[name] = &tclCmd{fn: fn, proc: false}
}

// Evaluate an argument, and catch any errors.
func cmdCatch(tcl *Tcl, args []string) int {
	if len(args) < 2 || len(args) > 3 {
		return tcl.SetResult(RetError, "catch script ?varName")
	}
	ret := tcl.eval(args[1], parserOptions{})
	if len(args) == 3 {
		tcl.SetVarValue(args[2], tcl.result)
	}
	if ret != RetOk {
		return tcl.SetResult(RetOk, "1")
	}
	return tcl.SetResult(RetOk, "0")
}

// Return Error condition.
func cmdError(tcl *Tcl, args []string) int {
	if len(args) != 2 {
		return tcl.SetResult(RetError, "error message")
	}
	return tcl.SetResult(RetError, args[1])
}

// Set command set name ?value.
func cmdSet(tcl *Tcl, args []string) int {
	if len(args) < 1 || len(args) > 3 {
		return tcl.SetResult(RetError, "set name ?value")
	}
	name := args[1]
	if len(args) > 2 {
		tcl.SetVarValue(name, args[2])
	}
	ret, result := tcl.GetVarValue(name)
	return tcl.SetResult(ret, result)
}

// Unset a list of variables.
func cmdUnSet(tcl *Tcl, args []string) int {
	for i := 1; i < len(args); i++ {
		tcl.UnSetVar(args[i])
	}
	return RetOk
}

// Substitute variables in arguments.
func cmdSubst(tcl *Tcl, args []string) int {
	opts := parserOptions{noEval: true, subst: true}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-nobacklashes":
			opts.noEscapes = true
		case "-novariables":
			opts.noVars = true
		case "-nocommands":
			opts.noCommands = true
		default:
			return tcl.eval(args[i], opts)
		}
	}
	return tcl.SetResult(RetError, "subst string")
}

// Print a string to standard output.
func cmdPuts(tcl *Tcl, args []string) int {
	text := args[1]
	fmt.Println(text)
	return tcl.SetResult(RetOk, "")
}

// Run a user process.
func userProc(tcl *Tcl, args []string, params string, body string) int {
	newenv := tcl.newEnv()
	// Current argument number.
	argNum := 1

	argList := tcl.ParseArgs(params)
	if len(argList) > 0 {
		for i, name := range argList {
			if name == "" {
				break
			}
			if name == "args" && i == len(argList) {
				tcl.setVarNewEnv(newenv, "args", strings.Join(args[argNum:], " "), true)
				break
			}
			tcl.setVarNewEnv(newenv, name, args[argNum], true)
			argNum++
		}
	}

	newenv.args = strings.Join(args, " ")
	// Switch to new environment and evaluate body of function.
	tcl.pushEnv(newenv)
	ret := tcl.eval(body, parserOptions{})
	tcl.popEnv()
	if ret == RetReturn {
		ret = RetOk
	}
	return ret
}

// Create a user procedure.
func cmdProc(tcl *Tcl, args []string) int {
	if len(args) != 4 {
		return tcl.SetResult(RetError, "proc args body")
	}
	name := args[1]
	tcl.cmds[name] = &tclCmd{
		fn:   func(t *Tcl, arg []string) int { return userProc(t, arg, args[2], args[3]) },
		proc: true,
		args: args[2],
		body: args[3],
	}
	return tcl.SetResult(RetOk, "")
}

// Create link to variable in current environment.
func cmdUpVar(tcl *Tcl, args []string) int {
	if len(args) < 3 {
		return tcl.SetResult(RetError, "upvar ?level varname var ?otherVar myVar")
	}
	v := 1
	level := 1
	top := false

	if len(args) > 3 {
		v++
		lvlNum := args[1]
		pos := 0
		if lvlNum[0] == '#' {
			top = true
			pos = 1
		}
		lvl, _, ok := ConvertStringToNumber(lvlNum, 10, pos)
		if !ok {
			return tcl.SetResult(RetError, "not valid level number")
		}
		level = lvl
	}

	env := tcl.getLevel(top, level)
	if env == nil {
		return tcl.SetResult(RetError, "not valid level number")
	}

	// Get a variable at a environment.
	for (v + 1) < len(args) {
		variable, ok := env.vars[args[v]]
		if ok {
			tcl.env.vars[args[v+1]] = variable
			tcl.env.local[args[v+1]] = false
		}
		v += 2
	}
	return tcl.SetResult(RetOk, "")
}

// Create link to variable in current environment.
func cmdUpLevel(tcl *Tcl, args []string) int {
	if len(args) < 2 {
		return tcl.SetResult(RetError, "uplevel ?level arg ?arg")
	}
	v := 1
	level := 1
	top := false
	cmd := 1

	if len(args) > 2 {
		v++
		lvlNum := args[1]
		pos := 0
		if lvlNum[0] == '#' {
			top = true
			pos = 1
		}
		lvl, _, ok := ConvertStringToNumber(lvlNum, 10, pos)
		if top && !ok {
			return tcl.SetResult(RetError, "not valid level number")
		}
		if ok {
			cmd++
			level = lvl
		}
	}

	// Must have at least one argument
	if len(args) < cmd {
		return tcl.SetResult(RetError, "uplevel ?level arg ?arg")
	}

	str := strings.Join(args[cmd:], " ")

	env := tcl.getLevel(top, level)
	if env == nil {
		return tcl.SetResult(RetError, "not valid level number")
	}

	// Save the current environment
	saveEnv := tcl.env
	saveLevel := tcl.level
	tcl.env = env // Set environment to level.
	tcl.level = env.level

	// Execute at level
	ret := tcl.eval(str, parserOptions{})

	// Restore symbol table and level.
	tcl.env = saveEnv
	tcl.level = saveLevel

	return ret
}

// Link to global variables in top level.
func cmdGlobal(tcl *Tcl, args []string) int {
	if tcl.level == 0 {
		return tcl.SetResult(RetOk, "")
	}
	if len(args) < 2 {
		return tcl.SetResult(RetError, "global varName ?varName")
	}
	env := tcl.getLevel(true, 0)
	if env == nil {
		return tcl.SetResult(RetError, "no top level?")
	}

	// Get a variable at a environment.
	for v := 1; v < len(args); v++ {
		variable, ok := env.vars[args[v]]
		if !ok {
			return tcl.SetResult(RetError, "variable "+args[v]+" not found")
		}
		tcl.env.vars[args[v]] = variable
	}
	return tcl.SetResult(RetOk, "")
}

// Create variables.
func cmdVariable(tcl *Tcl, args []string) int {
	if len(args) < 2 {
		return tcl.SetResult(RetError, "variable name ?value")
	}

	// Get a variable at a environment.
	for v := 1; v < len(args); v += 2 {
		if (v + 1) < len(args) {
			tcl.SetVarValue(args[v], args[v+1])
		}
	}
	return tcl.SetResult(RetOk, "")
}

// Rename a procedure.
func cmdRename(tcl *Tcl, args []string) int {
	if len(args) < 2 || len(args) > 3 {
		return tcl.SetResult(RetError, "rename OldName ?newName")
	}
	cmd, ok := tcl.cmds[args[1]]
	if !ok {
		return tcl.SetResult(RetError, "command "+args[1]+" not found")
	}
	tcl.cmds[args[1]] = nil
	if len(args) == 3 {
		tcl.cmds[args[2]] = cmd
	}
	return tcl.SetResult(RetOk, "")
}

var truthValue = map[string]bool{
	"":      false,
	"0":     false,
	"no":    false,
	"false": false,
	"1":     true,
	"yes":   true,
	"true":  true,
}

// Handle if {cond} {body} ?elseif {cond} {body} ?else {body}.
func cmdIf(tcl *Tcl, args []string) int {
	i := 3
	n := len(args)
	r := RetOk
	// Make sure arguments are correct.
	for i < n {
		if args[i] == "else" {
			break
		}
		if args[i] == "elseif" {
			i += 3
			continue
		}
		return tcl.SetResult(RetError, "if {} syntax error")
	}

	// Scan actual expression.
	i = 1
	for i < n {
		cond := "expr " + args[i]
		r = tcl.eval(cond, parserOptions{})
		if r != RetOk {
			break
		}
		v, ok := truthValue[tcl.result]
		if v || !ok {
			r = tcl.eval(args[i+1], parserOptions{})
			break
		}
		i += 2
		if i >= n {
			break
		}
		if args[i] == "elseif" {
			i++
			continue
		}
		if args[i] == "else" {
			r = tcl.eval(args[i+1], parserOptions{})
			break
		}
	}
	return r
}

// Handle simple control flow commands.
func cmdExit(tcl *Tcl, args []string) int {
	if len(args) == 1 {
		return tcl.SetResult(RetExit, "0")
	}
	return tcl.SetResult(RetExit, args[1])
}

// Handle simple control flow commands.
func cmdReturn(tcl *Tcl, args []string) int {
	switch len(args) {
	case 1:
		return tcl.SetResult(RetReturn, "")

	case 2:
		return tcl.SetResult(RetReturn, args[1])

	default:
		return tcl.SetResult(RetError, "wrong number of arguments to return")
	}
}

// Handle while {cond} {body}.
func cmdWhile(tcl *Tcl, args []string) int {
	if len(args) != 3 {
		tcl.result = "while {cond} {body}"
		return RetError
	}
	cond := "expr " + args[1]
	body := args[2]
	r := RetOk
	for {
		r = tcl.eval(cond, parserOptions{})
		if r != RetOk {
			break
		}
		v, ok := truthValue[tcl.result]
		if !v || !ok {
			break
		}
		r = tcl.eval(body, parserOptions{})
		if r != RetContinue && r != RetOk {
			if r == RetBreak {
				r = RetOk
			}
			break
		}
	}
	tcl.result = ""
	return r
}

// Handle for {init} {cond} {incr} {body}.
func cmdFor(tcl *Tcl, args []string) int {
	if len(args) != 5 {
		tcl.result = "for {init} {cond} {incr} {body}"
		return RetError
	}
	// Do initialization.
	r := tcl.eval(args[1], parserOptions{})
	if r != RetOk {
		return r
	}
	cond := "expr " + args[2]
	incr := args[3]
	body := args[4]
	for {
		r = tcl.eval(cond, parserOptions{})
		if r != RetOk {
			break
		}
		v, ok := truthValue[tcl.result]
		if !v || !ok {
			break
		}
		r = tcl.eval(body, parserOptions{})
		switch r {
		case RetOk, RetContinue:
		case RetBreak:
			return RetOk
		default:
			return r
		}
		r = tcl.eval(incr, parserOptions{})
		if r != RetOk {
			break
		}
	}
	tcl.result = ""
	return r
}

// Process switch statement.
func cmdSwitch(tcl *Tcl, args []string) int {
	exact := true
	regexpr := false
	i := 1
outer:
	for ; i < len(args); i++ {
		switch args[i] {
		case "-exact":
			exact = true
			regexpr = false
		case "-glob":
			exact = false
			regexpr = false
		case "-regexp":
			regexpr = true
		case "--":
			i++
			break outer
		default:
			break outer
		}
	}
	if i >= len(args) {
		return tcl.SetResult(RetError, "switch body")
	}
	str := args[i]
	var matchList []string
	i++
	// If at last element of string.
	switch {
	case (i + 1) == len(args):
		matchList = tcl.ParseArgs(args[i])
	case (i + 2) < len(args):
		matchList = args[i:]
	default:
		return tcl.SetResult(RetError, "switch body")
	}

	// Scan list in pairs.
	for i := 0; i < len(matchList); i += 2 {
		if matchList[i] == "default" {
			return tcl.eval(matchList[i+1], parserOptions{})
		}
		match := false
		switch {
		case regexpr:
			m, err := regexp.MatchString(matchList[i], str)
			if err != nil {
				return tcl.SetResult(RetError, err.Error())
			}
			match = m

		case exact:
			match = matchList[i] == str

		default:
			m := Match(matchList[i], str, false, len(str))
			if m < 0 {
				return tcl.SetResult(RetError, "Nesting level exceeded")
			}
			match = m != 0
		}
		if match {
			// If body is "-", use next body.
			for i <= len(matchList) && matchList[i+1] == "-" {
				i += 2
			}
			return tcl.eval(matchList[i+1], parserOptions{})
		}
	}
	return tcl.SetResult(RetOk, "")
}

var relationOprs = map[string]bool{
	">":  true,
	">=": true,
	"<":  true,
	"<=": true,
	"==": true,
	"!=": true,
}

func stringCmp(tcl *Tcl, args []string) int {
	cmp := strings.Compare(args[1], args[3])
	ret := "0"
	switch args[2] {
	case ">":
		if cmp > 0 {
			ret = "1"
		}
	case ">=":
		if cmp >= 0 {
			ret = "1"
		}

	case "<":
		if cmp < 0 {
			ret = "1"
		}
	case "<=":
		if cmp <= 0 {
			ret = "1"
		}
	case "==":
		if cmp == 0 {
			ret = "1"
		}
	case "!=":
		if cmp != 0 {
			ret = "1"
		}
	default:
	}
	return tcl.SetResult(RetOk, ret)
}

// Compute binary operation.
func binaryOp(tcl *Tcl, opr string, aval int, bval int) int {
	switch opr {
	case "+":
		aval += bval
	case "-":
		aval -= bval
	case "*":
		aval *= bval
	case "/":
		aval /= bval
	case "and":
		aval &= bval
	case "or":
		aval |= bval
	case "xor":
		aval ^= bval
	case "max":
		if aval < bval {
			aval = bval
		}
	case "min":
		if aval > bval {
			aval = bval
		}
	case ">":
		if aval > bval {
			aval = 1
		} else {
			aval = 0
		}
	case ">=":
		if aval >= bval {
			aval = 1
		} else {
			aval = 0
		}

	case "<":
		if aval < bval {
			aval = 1
		} else {
			aval = 0
		}
	case "<=":
		if aval <= bval {
			aval = 1
		} else {
			aval = 0
		}
	case "==":
		if aval == bval {
			aval = 1
		} else {
			aval = 0
		}
	case "!=":
		if aval != bval {
			aval = 1
		} else {
			aval = 0
		}
	default:
		return tcl.SetResult(RetError, "invalid operator")
	}

	// Convert result back to string.
	return tcl.SetResult(RetOk, ConvertNumberToString(aval, 10))
}

// Handle expr command.
func cmdMath(tcl *Tcl, args []string) int {
	// Join all arguments amd scan them ourselves.
	str := strings.ToLower(strings.Join(args[1:], " "))
	r := tcl.eval(str, parserOptions{noEval: true})
	if r != RetOk {
		return r
	}

	// Results of eval.
	str = tcl.result
	opr := ""

	// Try to convert first item to number.
	aval, pos, binary := ConvertStringToNumber(str, 10, 0)
	bval := 0

	if !binary && len(args) == 4 && relationOprs[args[2]] {
		return stringCmp(tcl, args)
	}

	// Skip space.
	for pos < len(str) {
		c := rune(str[pos])
		if !unicode.IsSpace(c) {
			break
		}
		pos++
	}

	// Collect operator.
	for pos < len(str) {
		c := rune(str[pos])
		if unicode.IsDigit(c) || unicode.IsSpace(c) {
			break
		}
		pos++
		opr += string(c)
	}

	// If no operators, and passed end of string, return error.
	if opr == "" {
		if binary {
			return tcl.SetResult(RetOk, ConvertNumberToString(aval, 10))
		}
		return tcl.SetResult(RetError, "operator not specified")
	}

	// Convert 2nd or 3rd as number.
	v, _, ok := ConvertStringToNumber(tcl.result, 10, pos)
	if !ok {
		if len(args) == 4 && relationOprs[args[2]] {
			return stringCmp(tcl, args)
		}

		return tcl.SetResult(RetError, "not a number")
	}
	bval = v

	if binary {
		return binaryOp(tcl, opr, aval, bval)
	}
	switch opr {
	case "-", "neg":
		aval = -bval
	case "not":
		if bval == 0 {
			aval = 1
		} else {
			aval = 0
		}
	case "inv":
		aval = ^bval
	case "abs":
		if bval < 0 {
			aval = -aval
		}
	case "bool":
		if bval != 0 {
			aval = 1
		}
	case "+", "":
	default:
		return tcl.SetResult(RetError, "invalid operator")
	}

	// Convert result back to string.
	return tcl.SetResult(RetOk, ConvertNumberToString(aval, 10))
}

// incr var ?amount.
func cmdIncr(tcl *Tcl, args []string) int {
	r, value := tcl.GetVarValue(args[1])
	if r != RetOk {
		return r
	}
	aval, _, ok := ConvertStringToNumber(value, 10, 0)
	if !ok {
		tcl.result = "not a number"
		return RetError
	}
	incr := 1
	if len(args) == 3 {
		incr, _, ok = ConvertStringToNumber(args[2], 10, 0)
		if !ok {
			tcl.result = "increment not a number"
			return RetError
		}
	}
	result := ConvertNumberToString(aval+incr, 10)
	tcl.SetVarValue(args[1], result)
	return RetOk
}

// decr var ?amount.
func cmdDecr(tcl *Tcl, args []string) int {
	r, value := tcl.GetVarValue(args[1])
	if r != RetOk {
		return r
	}
	aval, _, ok := ConvertStringToNumber(value, 10, 0)
	if !ok {
		tcl.result = "not a number"
		return RetError
	}
	decr := 1
	if len(args) == 3 {
		decr, _, ok = ConvertStringToNumber(args[2], 10, 0)
		if !ok {
			tcl.result = "decrement not a number"
			return RetError
		}
	}
	result := ConvertNumberToString(aval+decr, 10)
	tcl.SetVarValue(args[1], result)
	return RetOk
}

// Concatenate all arguments to one string.
func cmdConcat(tcl *Tcl, args []string) int {
	for i := range args {
		args[i] = strings.TrimSpace(args[i])
	}
	return tcl.SetResult(RetOk, strings.Join(args[1:], " "))
}

// Join list items in first argument, with second argument.
func cmdJoin(tcl *Tcl, args []string) int {
	if len(args) == 0 || len(args) > 3 {
		tcl.result = "join list ?string"
		return RetError
	}
	list := tcl.ParseArgs(args[1])
	join := " "
	if len(args) == 3 {
		join = args[2]
	}
	return tcl.SetResult(RetOk, strings.Join(list, join))
}

// Evaluate arguments.
func cmdEval(tcl *Tcl, args []string) int {
	return tcl.eval(strings.Join(args[1:], " "), parserOptions{})
}

// Append arguments to variable. append var ?args.
func cmdAppend(tcl *Tcl, args []string) int {
	if len(args) < 1 {
		tcl.result = "append name ?value"
		return RetError
	}
	name := args[1]
	str := strings.Join(args[2:], "")
	ret, result := tcl.GetVarValue(name)
	if ret == RetOk {
		str = result + str
	}

	tcl.SetVarValue(name, str)
	return tcl.SetResult(RetOk, str)
}

// Compare two arguments as strings.
func cmdEqual(tcl *Tcl, args []string) int {
	if len(args) != 3 {
		tcl.result = "eq a b"
		return RetError
	}
	if args[1] == args[2] {
		tcl.result = "1"
	} else {
		tcl.result = "0"
	}
	return RetOk
}

// Compare two arguments as strings.
func cmdNotEqual(tcl *Tcl, args []string) int {
	if len(args) != 3 {
		tcl.result = "ne a b"
		return RetError
	}
	if args[1] != args[2] {
		tcl.result = "1"
	} else {
		tcl.result = "0"
	}
	return RetOk
}
