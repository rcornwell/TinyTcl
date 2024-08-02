/*
 * TCL string and info functions.
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
	"strings"
	"unicode"
)

var funmap = map[string]func(*Tcl, []string) int{
	"compare":   stringCompare, // -nocase, -length int, string1, string2
	"equal":     stringCompare, // -nocase, -length int, string1, string2
	"first":     stringFind,    // needleString hayStack startIndex
	"last":      stringFind,    // needleString hayStack lastIndex
	"index":     stringIndex,   // string index
	"is":        stringIs,
	"length":    stringLength,
	"map":       stringMap,     // -nocase mapping string
	"match":     stringMatch,   // -nocase pattern string
	"range":     stringRange,   // string first last
	"repeat":    stringRepeat,  // string count
	"replace":   stringReplace, // string first last ?newstring
	"tolower":   stringToCase,  // string ?first ??last
	"totitle":   stringToCase,  // string ?first ??last
	"toupper":   stringToCase,  // string ?first ??last
	"trim":      stringTrim,    // string ?chars
	"trimleft":  stringTrim,    // string ?chars
	"trimright": stringTrim,    // string ?chars
}

func cmdString(tcl *Tcl, args []string, _ []string) int {
	if len(args) < 2 {
		return tcl.SetResult(RetError, "string function")
	}
	fn, ok := funmap[args[1]]
	if !ok {
		return tcl.SetResult(RetError, "string unknown function")
	}
	return fn(tcl, args)
}

// Compare and Equal functions.
func stringCompare(tcl *Tcl, args []string) int {
	equal := false
	nocase := false
	length := -1

	if args[1] == "equal" {
		equal = true
	}

	i := 2
	for ; i < len(args); i++ {
		if args[i] == "-nocase" {
			nocase = true
		} else if args[i] == "-length" {
			i++
			fmt.Println("Lens ", args[i])
			l, _, ok := ConvertStringToNumber(args[i], 10, 0)
			if !ok {
				return tcl.SetResult(RetError, "count")
			}
			length = l
		} else {
			break
		}
	}

	// Trim strings to correct length.
	str1 := args[i]

	if length >= 0 {
		str1 = str1[:min(length, len(str1))]
	}

	str2 := args[i+1]
	if length >= 0 {
		str2 = str2[:min(length, len(str2))]
	}

	// convert to lower case if nocase match.
	if nocase {
		str1 = strings.ToLower(str1)
		str2 = strings.ToLower(str2)
	}

	res := strings.Compare(str1, str2)
	if equal {
		if res == 0 {
			res = 1
		} else {
			res = 0
		}
	}

	return tcl.SetResult(RetOk, ConvertNumberToString(res, 10))
}

// Find character in string.
func stringFind(tcl *Tcl, args []string) int {
	if len(args) > 5 {
		return tcl.SetResult(RetError, "string "+args[1]+"needlestring haystack ?startindex")
	}
	str := args[2]   // String to find.
	match := args[3] // String to search in.
	index := 0
	dir := 1
	maxlen := len(match) - len(str)

	// If startindex set step to position to start.
	if len(args) == 5 {
		i, _, ok := convertListIndex(args[4], len(str), 0)
		if !ok {
			return tcl.SetResult(RetError, "index invalid")
		}
		index = max(min(i, maxlen), 0)
	}

	// If last reverse search.
	if args[1] == "last" {
		dir = -1
	}

	for index > 0 && index <= maxlen {
		if str == match[index:index+len(str)] {
			return tcl.SetResult(RetOk, ConvertNumberToString(index, 10))
		}
		index += dir
	}
	return tcl.SetResult(RetOk, "-1")
}

// Return character at index.
func stringIndex(tcl *Tcl, args []string) int {
	if len(args) > 4 {
		return tcl.SetResult(RetError, "string index string index")
	}
	str := args[2]
	index, _, ok := convertListIndex(args[3], len(str), 0)
	if !ok {
		return tcl.SetResult(RetError, "index invalid")
	}
	if index < 0 || index >= len(str) {
		return tcl.SetResult(RetOk, "")
	}
	return tcl.SetResult(RetOk, string(str[index]))
}

// See if a character matches a class.
func stringIs(tcl *Tcl, args []string) int {
	if len(args) < 4 {
		return tcl.SetResult(RetError, "string is class ?-strict ?-failindex varname? string")
	}
	strict := false
	fail := ""
	class := args[2]
	i := 3
first:
	for ; i < len(args); i++ {
		switch args[i] {
		case "-strict":
			strict = true
		case "-failindex":
			fail = args[i+1]
			i++
		default:
			break first
		}
	}

	if len(args[i]) == 0 {
		if strict {
			return tcl.SetResult(RetOk, "0")
		}
		return tcl.SetResult(RetOk, "1")
	}

	index := 0
	ch := rune(0)
	ok := true

outer:
	for index, ch = range args[i] {
		switch class {
		case "alnum":
			if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) {
				ok = false
				break outer
			}

		case "alpha":
			if !unicode.IsLetter(ch) {
				ok = false
				break outer
			}

		case "ascii":
			if byte(ch) >= 0x80 {
				ok = false
				break outer
			}

		case "boolean":
			_, tok := truthValue[args[i]]
			if tok {
				return tcl.SetResult(RetOk, "1")
			}
			return tcl.SetResult(RetOk, "0")

		case "control":
			if !unicode.IsControl(ch) {
				ok = false
				break outer
			}

		case "digit":
			if !unicode.IsDigit(ch) {
				ok = false
				break outer
			}

		case "false":
			v, tok := truthValue[args[i]]
			if tok && !v {
				return tcl.SetResult(RetOk, "1")
			}
			return tcl.SetResult(RetOk, "0")

		case "graphic":
			if !unicode.IsGraphic(ch) {
				ok = false
				break outer
			}

		case "lower":
			if !unicode.IsLower(ch) {
				ok = false
				break outer
			}

		case "print":
			if !unicode.IsPrint(ch) {
				ok = false
				break outer
			}

		case "punct":
			if !unicode.IsPunct(ch) {
				ok = false
				break outer
			}

		case "space":
			if !unicode.IsSpace(ch) {
				ok = false
				break outer
			}

		case "true":
			v, tok := truthValue[args[i]]
			if tok && v {
				return tcl.SetResult(RetOk, "1")
			}
			return tcl.SetResult(RetOk, "0")

		case "upper":
			if !unicode.IsUpper(ch) {
				ok = false
				break outer
			}
		}
	}

	// Set completion code, and store position of mismatch if asked for.
	if !ok {
		if fail != "" {
			tcl.SetVarValue(fail, ConvertNumberToString(index, 10))
		}
		return tcl.SetResult(RetOk, "0")
	}
	return tcl.SetResult(RetOk, "1")
}

// Return length of string.
func stringLength(tcl *Tcl, args []string) int {
	if len(args) > 3 {
		return tcl.SetResult(RetError, "string length string")
	}
	return tcl.SetResult(RetOk, ConvertNumberToString(len(args[2]), 10))
}

// Translate string based on map.
func stringMap(tcl *Tcl, args []string) int {
	nocase := false

	i := 2
	if args[2] == "-nocase" {
		nocase = true
		i++
	}

	if len(args) > (i + 2) {
		return tcl.SetResult(RetError, "string map ?-nocase mapping string")
	}

	res := ""
	index := 0
	str := args[i+1]
	match := str
	mapping := tcl.ParseArgs(args[i])
	if nocase {
		match = strings.ToLower(match)
	}

	// Walk through the string and replace any matches found.
	for index < len(str) {
		replace := false
		for i := 0; i < len(mapping); i += 2 {
			if nocase {
				if strings.HasPrefix(match[index:], strings.ToLower(mapping[i])) {
					replace = true
				}
			} else if strings.HasPrefix(match[index:], mapping[i]) {
				replace = true
			}
			if replace {
				res += mapping[i+1]
				index += len(mapping[i])
				break
			}
		}
		if !replace {
			res += string(str[index])
			index++
		}
	}
	return tcl.SetResult(RetOk, res)
}

// Glob match a string.
func stringMatch(tcl *Tcl, args []string) int {
	nocase := false

	i := 2
	if args[2] == "-nocase" {
		nocase = true
		i++
	}

	if len(args) > (i + 2) {
		return tcl.SetResult(RetError, "string match ?-nocase pattern string")
	}

	res := Match(args[i], args[i+1], nocase, len(args[i+1]))
	if res < 0 {
		return tcl.SetResult(RetError, "match depth exceeded")
	}
	return tcl.SetResult(RetOk, ConvertNumberToString(res, 10))
}

// Return characters between first and last index.
func stringRange(tcl *Tcl, args []string) int {
	if len(args) > 5 {
		return tcl.SetResult(RetError, "string range string first last")
	}
	str := args[2]
	first, _, fok := convertListIndex(args[3], len(str), 0)
	if !fok {
		return tcl.SetResult(RetError, "first index invalid")
	}

	last, _, lok := convertListIndex(args[4], len(str), 0)
	if !lok {
		return tcl.SetResult(RetError, "last index invalid")
	}

	first = max(0, first)
	last = min(last, len(str))
	if last < 0 || first > last {
		return tcl.SetResult(RetOk, "")
	}
	return tcl.SetResult(RetOk, str[first:last+1])
}

// Repeat a string number of times.
func stringRepeat(tcl *Tcl, args []string) int {
	if len(args) > 4 {
		return tcl.SetResult(RetError, "string repeat string count")
	}
	str := args[2]
	count, _, ok := ConvertStringToNumber(args[3], 10, 0)
	if ok {
		return tcl.SetResult(RetError, "count invalid "+args[3])
	}

	if count <= 0 {
		return tcl.SetResult(RetOk, "")
	}

	result := ""
	for range count {
		result += str
	}
	return tcl.SetResult(RetOk, result)
}

// Replace range of characters in string with new string.
func stringReplace(tcl *Tcl, args []string) int {
	if len(args) > 6 {
		return tcl.SetResult(RetError, "string replace string first last ?newstring")
	}
	str := args[2]
	newstr := ""
	if len(args) > 5 {
		newstr = args[5]
	}
	first, _, fok := convertListIndex(args[3], len(str), 0)
	if !fok {
		return tcl.SetResult(RetError, "first index invalid")
	}

	last, _, lok := convertListIndex(args[4], len(str), 0)
	if !lok {
		return tcl.SetResult(RetError, "last index invalid")
	}
	first = max(0, first)
	last = min(last, len(str))
	if last < 0 || first > last {
		return tcl.SetResult(RetOk, str)
	}

	result := str[:first]
	result += newstr
	result += str[last+1:]

	return tcl.SetResult(RetOk, result)
}

func stringToCase(tcl *Tcl, args []string) int {
	first := 0
	last := len(args[2])
	switch len(args) {
	case 3:
	case 4:
		i, _, ok := convertListIndex(args[3], last, 0)
		if !ok {
			return tcl.SetResult(RetError, "first invalid")
		}
		first = i
		last = i
	case 5:
		f, _, fok := convertListIndex(args[3], last, 0)
		if !fok {
			return tcl.SetResult(RetError, "first invalid")
		}

		l, _, lok := convertListIndex(args[4], last, 0)
		if !lok {
			return tcl.SetResult(RetError, "last invalid")
		}
		first = f
		last = l
	default:
		return tcl.SetResult(RetError, "string "+args[1]+" string ?first ?last")
	}

	last++
	first = max(min(first, len(args[2])), 0)
	last = max(min(last, len(args[2])), 0)
	str := args[2]
	res := str[0:first]
	switch args[1] {
	case "tolower":
		res += strings.ToLower(str[first:last])
	case "toupper":
		res += strings.ToUpper(str[first:last])
	case "totitle":
		last = first + 1
		res += strings.ToTitle(str[first:last])
	}
	res += str[last:]
	return tcl.SetResult(RetOk, res)
}

// Trim leading of trailing charcters from a string.
func stringTrim(tcl *Tcl, args []string) int {
	match := " \t\n\r"
	if len(args) > 4 {
		return tcl.SetResult(RetError, "string "+args[1]+" string ?chars")
	}

	if len(args) == 4 {
		match = args[3]
	}

	res := args[2]
	start := 0
	last := len(res) - 1
	if args[1] != "trimright" {
		for start < len(res) {
			if !strings.ContainsAny(string(res[start]), match) {
				break
			}
			start++
		}
	}

	if args[1] != "trimleft" {
		for last >= 0 && last >= start {
			if !strings.ContainsAny(string(res[last]), match) {
				last++
				break
			}
			last--
		}
	}

	return tcl.SetResult(RetOk, res[start:last])
}

// Report on various information about the system.
func cmdInfo(tcl *Tcl, args []string, _ []string) int {
	if len(args) < 2 {
		return tcl.SetResult(RetError, "info function")
	}
	list := []string{}
	switch args[1] {
	case "args": // info args procname
		if len(args) != 3 {
			return tcl.SetResult(RetError, "info body procname")
		}
		cmd := tcl.cmds[args[2]]
		if !cmd.proc {
			return tcl.SetResult(RetError, args[2]+" not a proc")
		}
		return tcl.SetResult(RetOk, cmd.arg[2])

	case "body": // info body procname
		if len(args) != 3 {
			return tcl.SetResult(RetError, "info body procname")
		}
		cmd := tcl.cmds[args[2]]
		if !cmd.proc {
			return tcl.SetResult(RetError, args[2]+" not a proc")
		}
		return tcl.SetResult(RetOk, cmd.arg[3])

	case "commands": // info commands ?pattern
		list = tcl.listCommands(false)

	case "exists": // info exists varName
		if len(args) < 4 {
			err, _ := tcl.GetVarValue(args[2])
			if err == RetOk {
				return tcl.SetResult(RetOk, "1")
			}
		}
		return tcl.SetResult(RetOk, "0")

	case "globals": // info globals ?pattern
		list = tcl.listGlobals()

	case "level": // info level ?number?
		if len(args) > 3 {
			num, _, ok := ConvertStringToNumber(args[2], 10, 0)
			if !ok {
				return tcl.SetResult(RetError, "invalid level")
			}
			top := true
			if num < 0 {
				top = false
				num = -num
			}
			env := tcl.getLevel(top, num)
			return tcl.SetResult(RetOk, env.args)
		}
		return tcl.SetResult(RetOk, ConvertNumberToString(tcl.level, 10))

	case "locals": // info locals ?pattern
		list = tcl.listVars(true)

	case "procs": // info procs ?pattern
		list = tcl.listCommands(true)

	case "vars": // info vars ?pattern
		list = tcl.listVars(false)
	}
	if len(args) > 2 {
		list = matchPattern(list, args[2])
	}
	return tcl.SetResult(RetOk, strings.Join(list, " "))
}

// Return list of variables.
func (tcl *Tcl) listVars(local bool) []string {
	res := []string{}

	for v := range tcl.env.vars {
		if (local && tcl.env.local[v]) || !local {
			res = append(res, v)
		}
	}
	return res
}

// Return list of global.
func (tcl *Tcl) listGlobals() []string {
	res := []string{}

	env := tcl.getLevel(true, 0)
	for v := range env.vars {
		res = append(res, v)
	}
	return res
}

// Return list of commands.
func (tcl *Tcl) listCommands(user bool) []string {
	res := []string{}

	for v := range tcl.cmds {
		if (user && tcl.cmds[v].proc) || !user {
			res = append(res, v)
		}
	}
	return res
}

// Return list of items matching pattern.
func matchPattern(list []string, pattern string) []string {
	res := []string{}

	for _, n := range list {
		r := Match(pattern, n, false, len(n))
		if r == 1 {
			res = append(res, n)
		}
	}
	return res
}
