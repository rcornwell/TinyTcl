/*
 * TCL  list processing commands.
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
	"regexp"
	"strings"
	"unicode"
)

// Join list items in first argument, with second argument.
func cmdList(tcl *Tcl, args []string) int {
	str := ""

	for _, item := range args[1:] {
		str += " " + StringEscape(item)
	}
	return tcl.SetResult(RetOk, str[1:])
}

// Return the number of elements in a list.
func cmdLLength(tcl *Tcl, args []string) int {
	if len(args) != 2 {
		tcl.result = "llength list"
		return RetError
	}
	if args[1] == "" {
		return tcl.SetResult(RetOk, "0")
	}
	list := tcl.ParseArgs(args[1])

	return tcl.SetResult(RetOk, ConvertNumberToString(len(list), 10))
}

// Convert list index string to number.
func convertListIndex(str string, listMax int, pos int) (int, int, bool) {
	result := 0
	r := false
	end := false
	// Skip leading spaces.
	for unicode.IsSpace(rune(str[pos])) && pos < len(str) {
		pos++
	}
	if pos >= len(str) {
		return 0, pos, r
	}

	if strings.HasPrefix(str[pos:], "end") {
		end = true
		pos += 3
		if pos >= len(str) || str[pos] != '-' {
			return listMax - 1, pos, true
		}
		pos++
	}

	if pos >= len(str) {
		return 0, pos, r
	}

	for pos < len(str) {
		d := strings.Index(hex, strings.ToLower(string(str[pos])))
		if d == -1 || d >= 10 {
			break
		}
		result = (result * 10) + d
		r = true
		pos++
	}

	// Convert index into correct position.
	if r && end {
		result = listMax - result - 1
	}

	return result, pos, r
}

// Returns selected elements from a list.
func cmdLIndex(tcl *Tcl, args []string) int {
	if len(args) < 2 {
		tcl.result = "lindex list ?index"
		return RetError
	}
	if len(args) < 3 {
		return tcl.SetResult(RetOk, args[1])
	}
	res := []string{"list"}
	list := tcl.ParseArgs(args[1])
	pending := false // Indicate if we have any pending appends.
	for i := 2; i < len(args); i++ {
		index := args[i]
		if index == "" {
			res = append(res, list...)
			pending = false
			break
		}
		pos := 0
		for pos < len(index) {
			i, npos, ok := convertListIndex(index, len(list), pos)
			if !ok {
				break
			}
			if i < len(list) {
				list = tcl.ParseArgs(list[i])
			}
			pos = npos
			pending = true
		}
	}
	if pending {
		res = append(res, list...)
	}

	// Let list build result.
	return cmdList(tcl, res)
}

// Returns list starting at first and ending at last.
func cmdLRange(tcl *Tcl, args []string) int {
	if len(args) < 4 {
		tcl.result = "lrange list first last"
		return RetError
	}
	list := tcl.ParseArgs(args[1])
	first, _, fok := convertListIndex(args[2], len(list), 0)
	if !fok {
		return tcl.SetResult(RetError, "lrange first index invalid")
	}
	last, _, lok := convertListIndex(args[3], len(list), 0)
	if !lok {
		return tcl.SetResult(RetError, "lrange second index invalid")
	}

	// Convert indexes to valid ranges.
	first = max(min(first, len(list)), 0)
	last = max(min(last, len(list)), 0)
	res := []string{"list"}
	res = append(res, list[first:last+1]...)

	// Let list build result.
	return cmdList(tcl, res)
}

// Appends a list to an existing list.
func cmdLAppend(tcl *Tcl, args []string) int {
	if len(args) < 2 {
		tcl.result = "lappend list ?values"
		return RetError
	}
	name := args[1]
	str := strings.Join(args[2:], " ")
	ret, result := tcl.GetVarValue(name)
	if ret == RetOk && result != "" {
		str = result + " " + str
	}

	tcl.SetVarValue(name, str)
	return tcl.SetResult(RetOk, str)
}

// Insert a list into a list.
func cmdLInsert(tcl *Tcl, args []string) int {
	if len(args) < 3 {
		tcl.result = "linsert list index ?values"
		return RetError
	}
	list := tcl.ParseArgs(args[1])
	index, _, ok := convertListIndex(args[2], len(list)+1, 0)
	if !ok {
		return tcl.SetResult(RetError, "index not valid")
	}
	index = max(min(index, len(list)), 0)
	newList := []string{"list"}
	newList = append(newList, list[:index]...)
	newList = append(newList, args[3:]...)
	newList = append(newList, list[index:]...)

	// Let List put things back together.
	return cmdList(tcl, newList)
}

// Replace elements of list with new elements.
func cmdLReplace(tcl *Tcl, args []string) int {
	if len(args) < 3 {
		tcl.result = "lreplace list first last ?elements"
		return RetError
	}
	list := tcl.ParseArgs(args[1])
	first, _, fok := convertListIndex(args[2], len(list), 0)
	if !fok {
		return tcl.SetResult(RetError, "lreplace first index invalid")
	}
	last, _, lok := convertListIndex(args[3], len(list), 0)
	if !lok {
		return tcl.SetResult(RetError, "lreplce second index invalid")
	}

	first = max(min(first, len(list)), 0)
	last = max(min(last, len(list)), 0)
	newList := []string{"list"}
	newList = append(newList, list[:first]...)
	if len(args) > 3 {
		newList = append(newList, args[4:]...)
	}
	newList = append(newList, list[last+1:]...)

	// Let List put things back together.
	return cmdList(tcl, newList)
}

// Compare two elements.
func (tcl *Tcl) order(integer bool, reverse bool, command, a, b string) (bool, int) {
	r := false

	// If we have compare function use that.
	if command != "" {
		str := command + " " + StringEscape(a) + " " + StringEscape(b)
		str += " " + b

		ret := tcl.eval(str, parserOptions{})
		if ret != RetOk {
			return false, ret
		}

		v, _, ok := ConvertStringToNumber(tcl.GetResult(), 10, 0)
		if !ok {
			return false, RetError
		}
		if reverse {
			v = -1 * v
		}
		return v < 0, RetOk
	}

	// If not do integer or string compare
	if integer {
		ia, _, aok := ConvertStringToNumber(a, 10, 0)
		ib, _, bok := ConvertStringToNumber(b, 10, 0)
		if !aok || !bok {
			return false, RetError
		}
		r = ia < ib
	} else {
		r = a < b
	}
	return reverse != r, RetOk
}

// Sort a list.
func cmdLSort(tcl *Tcl, args []string) int {
	integer := false
	reverse := false
	command := ""
	i := 1
outer:
	for ; i < len(args); i++ {
		switch args[i] {
		case "-increasing":
			reverse = false
		case "-decreasing":
			reverse = true
		case "-ascii":
			integer = false
		case "-integer":
			integer = true
		case "-command":
			i++
			if i >= len(args) {
				return tcl.SetResult(RetError, "missing command argument")
			}
			command = args[i]
		default:
			break outer
		}
	}
	list := tcl.ParseArgs(args[i])
	k := 0
	for j := 1; j < len(list); j++ {
		key := list[j]
		k = j - 1
		for k >= 0 {
			ord, err := tcl.order(integer, reverse, command, key, list[k])
			if err != RetOk {
				return err
			}
			if !ord {
				break
			}
			list[k+1] = list[k]
			k--
		}
		list[k+1] = key
	}
	return cmdList(tcl, append([]string{"list"}, list...))
}

const (
	opGlob = iota + 1
	opExact
	opInteger
	opRegExp
)

// Sort a list.
func cmdLSearch(tcl *Tcl, args []string) int {
	op := opGlob
	all := false
	inline := false
	nocase := false
	not := false
	start := 0
	sort := false

	i := 1
outer:
	for ; i < len(args); i++ {
		switch args[i] {
		case "-integer":
			op = opInteger
		case "-glob":
			op = opGlob
		case "-exact":
			op = opExact
		case "-regexp":
			op = opRegExp
		case "-all":
			all = true
		case "-not":
			not = true
		case "-nocasa":
			nocase = true
		case "-inline":
			inline = true
		case "-sorted":
			sort = true
		case "-start":
			i++
			if i >= len(args) {
				return tcl.SetResult(RetError, "missing argument for start")
			}
			s, _, ok := ConvertStringToNumber(args[i], 10, 0)
			if !ok {
				return tcl.SetResult(RetError, "start option not a number")
			}
			start = s
		default:
			break outer
		}
	}
	if (i + 1) >= len(args) {
		return tcl.SetResult(RetError, "lsearch ?options list pattern")
	}
	list := tcl.ParseArgs(args[i])
	pattern := args[i+1]
	imatch := 0
	if op == opInteger {
		m, _, ok := ConvertStringToNumber(args[i+1], 10, 0)
		if !ok {
			return tcl.SetResult(RetError, "pattern not a number")
		}
		imatch = m
	}
	result := []string{}
matchLoop:
	for i := start; i < len(list); i++ {
		value := list[i]
		match := false
		switch op {
		case opGlob:
			m := Match(pattern, value, nocase, len(value))
			if m < 0 {
				return tcl.SetResult(RetError, "Nesting level exceeded")
			}
			match = m != 0

		case opExact:
			if nocase {
				match = strings.EqualFold(pattern, value)
			} else {
				match = pattern == value
			}

		case opRegExp:
			m, err := regexp.MatchString(pattern, value)
			if err != nil {
				return tcl.SetResult(RetError, err.Error())
			}
			match = m

		case opInteger:
			v, _, ok := ConvertStringToNumber(value, 10, 0)
			if !ok {
				return tcl.SetResult(RetError, "Not a number")
			}
			match = imatch == v
		}

		// Evaluate match.
		if not != match {
			if inline {
				result = append(result, value)
			} else {
				result = append(result, ConvertNumberToString(i, 10))
			}
			if !all {
				break matchLoop
			}
		}
	}

	if len(result) == 0 {
		return tcl.SetResult(RetOk, "-1")
	}

	// Sort result if asked for.
	// If not inline, then indices will always be in order.
	if sort && !inline {
		k := 0
		for j := 1; j < len(result); j++ {
			key := result[j]
			k = j - 1
			for k >= 0 {
				ord, err := tcl.order(op == opInteger, false, "", key, result[k])
				if err != RetOk {
					return err
				}
				if !ord {
					break
				}
				result[k+1] = result[k]
				k--
			}
			result[k+1] = key
		}
	}

	return cmdList(tcl, append([]string{"list"}, result...))
}

// Elements in a list.
func cmdLSet(tcl *Tcl, args []string) int {
	if len(args) < 3 {
		tcl.result = "lset varName ?index list"
		return RetError
	}
	newList := args[len(args)-1]

	if len(args) == 3 || args[2] == "" {
		tcl.SetVarValue(args[1], newList)
		return tcl.SetResult(RetOk, newList)
	}

	err, setList := tcl.GetVarValue(args[1])
	if err != RetOk {
		return err
	}

	type stackElement struct {
		list  []string
		index int
	}
	stack := []stackElement{}
	list := tcl.ParseArgs(setList)
	// Scan the list, getting lists to replace and their index.
	for i := 2; i < len(args)-1; i++ {
		index := args[i]
		pos := 0
		for pos < len(index) {
			i, npos, ok := convertListIndex(index, len(list), pos)
			if !ok {
				break
			}
			if i < 0 || i >= len(list) {
				return tcl.SetResult(RetError, "list index out of range")
			}
			stack = append(stack, stackElement{list: list, index: i})
			list = tcl.ParseArgs(list[i])
			pos = npos
		}
	}

	// Now work from bottom of list to top, replacing elements as called for.
	value := newList
	for i := len(stack) - 1; i >= 0; i-- {
		list = stack[i].list
		list[stack[i].index] = value
		str := ""

		// Construct the string that will be replaced in index list.
		for _, item := range list {
			item = StringEscape(item)
			str += " " + item
		}
		value = str[1:]
	}

	// Let list build result.
	return tcl.SetResult(RetOk, value)
}

// Split string based on delimiters.
func cmdSplit(tcl *Tcl, args []string) int {
	if len(args) < 2 || len(args) > 3 {
		return tcl.SetResult(RetError, "split string ?splitChars?")
	}

	// Create result as list.
	result := []string{"list"}
	current := ""
	match := ""
	if len(args) == 3 {
		match = args[2]
	}

	// Scan and make a list of none matching characters.
	for _, ch := range args[1] {
		switch {
		case len(args) == 2:
			current += string(ch)
		case match == "":
			result = append(result, string(ch))
			current = ""
		case strings.ContainsAny(string(ch), match):
			result = append(result, current)
			current = ""
		default:
			current += string(ch)
		}
	}

	// Append anything left over.
	if current != "" {
		result = append(result, current)
	}
	return cmdList(tcl, result)
}

// Foreach command.
func cmdForEach(tcl *Tcl, args []string) int {
	type argList struct {
		vars  []string // variables.
		list  []string // Values of list.
		index int      // Current index.
	}

	lists := []argList{}

	if len(args) < 4 {
		tcl.SetResult(RetError, "foreach var list body")
	}

	i := 1
	// Convert arguments up until last one to a pair of var/list pairs.
	for (i + 1) < len(args) {
		v := tcl.ParseArgs(args[i])
		l := tcl.ParseArgs(args[i+1])
		lists = append(lists, argList{vars: v, list: l, index: 0})
		i += 2
	}
	body := args[i]

outer:
	for {
		done := true
		// Loop over all lists.
		for i := range lists {
			// For each list pair, copy list elements to variables.
			for _, v := range lists[i].vars {
				ind := lists[i].index
				if ind < len(lists[i].list) {
					tcl.SetVarValue(v, lists[i].list[ind])
					done = false
					lists[i].index++
				} else {
					tcl.SetVarValue(v, "{}")
				}
			}
		}

		// If exhausted lists, done.
		if done {
			break
		}

		// Run body.
		r := tcl.eval(body, parserOptions{})
		switch r {
		case RetOk, RetContinue:
		case RetBreak:
			break outer
		default:
			return tcl.SetResult(r, "")
		}
	}

	return tcl.SetResult(RetOk, "")
}
