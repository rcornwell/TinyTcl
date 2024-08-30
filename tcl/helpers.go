/*
 * TCL internal functions.
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
	"strings"
	"unicode"
)

func hexStringToChar(str string) (byte, int) {
	n := strings.Index(hex, strings.ToLower(string(str[0])))
	if n < 0 {
		return 0, -2
	}

	val := byte(n)
	if len(str) < 2 {
		return val, 1
	}
	n = strings.Index(hex, strings.ToLower(string(str[1])))
	if n < 0 {
		return val, 1
	}
	val = (val << 4) | byte(n)
	return val, 2
}

// Process escape character.
func UnEscape(str string) (string, int) {
	if str == "" {
		return "", -1
	}

	result := ""
	num := 0
	digits := 0
	inEscape := false
	inOctal := false

	for pos := 0; pos < len(str); pos++ {
		ch := str[pos]
		if inOctal {
			if digits < 3 && ch >= '0' && ch <= '7' {
				num = (num << 3) + int(ch-'0')
				digits++
				continue
			}
			inOctal = false
			digits = 0
			result += string(rune(num))
		}

		if inEscape {
			switch ch {
			case '\n': // Multiline
			case '\\':
				result += "\\"

			case 'a':
				result += "\a"

			case 'b':
				result += "\b"

			case 'e':
				result += "\033"

			case 'f':
				result += "\f"

			case 'n':
				result += "\n"

			case 'r':
				result += "\r"

			case 't':
				result += "\t"

			case 'v':
				result += "\v"

			case 'x':
				if (pos + 1) > len(str) {
					return "", -2
				}
				val, n := hexStringToChar(str[pos+1:])
				if n < 0 {
					return "", n
				}
				if n != 0 {
					result += string(val)
					pos += n
				}

			case '0':
				inOctal = true
				num = 0

			default:
				result += string(ch)
			}
			inEscape = false
		} else {
			if ch == '\\' {
				inEscape = true
			} else {
				result += string(ch)
			}
		}
	}

	if digits > 0 {
		result += string(rune(num))
	}
	return result, 0
}

// Convert a TCL numeric string to number, return last position scanned and whether value converted.
func ConvertStringToNumber(str string, base int, pos int) (int, int, bool) {
	result := 0
	neg := false
	ok := false
	origPos := pos
	// Skip leading spaces.
	for pos < len(str) && unicode.IsSpace(rune(str[pos])) {
		pos++
	}
	if pos >= len(str) {
		return 0, origPos, ok
	}

	// Check if negative number.
	if str[pos] == '-' {
		neg = true
		pos++
	} else if str[pos] == '+' {
		pos++
	}
	if pos >= len(str) {
		return 0, origPos, ok
	}

	// Check if hex or octal.
	if str[pos] == '0' {
		ok = true
		base = 8
		pos++
		if pos >= len(str) {
			return 0, pos, ok
		}
		if str[pos] == 'x' || str[pos] == 'X' {
			base = 16
			pos++
		}
	}

	if pos >= len(str) {
		return 0, pos, ok
	}

	for pos < len(str) {
		d := strings.Index(hex, strings.ToLower(string(str[pos])))
		if d == -1 || d >= base {
			break
		}
		result = (result * base) + d
		ok = true
		pos++
	}
	if ok && neg {
		result = -result
	}
	if ok {
		return result, pos, ok
	}
	return result, origPos, ok
}

// Convert number back to a string.
func ConvertNumberToString(num int, base int) string {
	result := ""
	neg := false
	// Add based on base.
	switch base {
	case 8:
		result = "0"
	case 16:
		result = "0x"
	default:
		if num < 0 {
			neg = true
			num = -num
		}
	}

	// If number is zero append 0 and return.
	if num == 0 {
		if base != 8 {
			result += "0"
		}
		return result
	}

	// Prepends digits to number.
	for num != 0 {
		d := num % base
		result = string(hex[d]) + result
		num /= base
	}

	// Put negative sign if negative.
	if neg {
		result = "-" + result
	}
	return result
}

// Set a variable to value, create variable if it does not exist.
func (tcl *Tcl) SetVarValue(name string, value string) {
	variable, ok := tcl.env.vars[name]
	if !ok {
		variable = &tclVar{value: value}
		tcl.env.vars[name] = variable
	} else {
		variable.value = value
	}
}

// Remove a variable from current environment.
func (tcl *Tcl) UnSetVar(name string) {
	delete(tcl.env.vars, name)
	delete(tcl.env.local, name)
}

// Retrieve a value of a variable.
func (tcl *Tcl) GetVarValue(name string) (int, string) {
	variable, ok := tcl.env.vars[name]
	if !ok {
		return RetError, "value: " + name + " not found"
	}
	return RetOk, variable.value
}

// Does this string need to be escaped.
func StringEscape(str string) string {
	if str == "" {
		return "{}"
	}
	braces := 0
	sp := false
	hasBlank := false
	end := rune(0)
	for i, ch := range str {
		end = ch
		if strings.ContainsAny(string(ch), " \t\n\r\v[]$") {
			sp = true
			if braces == 0 {
				hasBlank = true
			}
		}
		if ch == '{' {
			braces++
		}
		if ch == '}' {
			braces--
		}
		if ch == '\\' {
			sp = true
			i++
			if i == len(str) {
				return "{" + str + "}"
			}
		}
	}

	if sp {
		if braces != 0 && !(str[0] == '{' && end == '}') {
			res := ""
			sp = false
			for _, ch := range str {
				if !sp && (ch == '{' || ch == '}') {
					res += "\\"
				}
				sp = ch == '\\'
				res += string(ch)
			}
			return res
		}
		if hasBlank || !(str[0] == '{' && end == '}') {
			return "{" + str + "}"
		}
	}

	return str
}

// Match a pattern. Return length of match or -1 if no match.
func Match(pat string, target string, ignoreCase bool, depth int) int {
	// If at end of string
	if pat == "" {
		if target == "" {
			return 0
		}
		return 1
	}

	// Check if depth exceeded.
	if depth == 0 {
		return -1
	}

	// Try and match.
	i := 0
	k := 0

outer:
	for i < len(pat) {
		switch pat[i] {
		case '*': // Match any number of character.
			r := Match(pat[i+1:], target[k:], ignoreCase, depth-1)
			if r > 0 {
				return r
			}
			if k >= len(target) {
				return 0
			}
			k++

		case '?': // Match any single character.
			if k >= len(target) {
				return 0
			}
			i++
			k++

		case '[': // Match any of the following characters.
			i++
			for i < len(pat) {
				first := pat[i]
				last := first
				i++
				if i > len(pat) {
					break outer
				}
				if first == ']' {
					break
				}
				if first == '\\' {
					first = pat[i]
					i++
					if i > len(pat) {
						break outer
					}
				}

				if pat[i] == '-' {
					i++
					if i > len(pat) {
						last = pat[i]
					}
				}
				if ignoreCase {
					if strings.ToLower(string(target[k])) < strings.ToLower(string(first)) ||
						strings.ToLower(string(target[k])) > strings.ToLower(string(last)) {
						return 0
					}
				} else {
					if target[k] < first || target[k] > last {
						return 0
					}
				}
			}
		case '\\': // Escape character.
			i++
			if i >= len(pat) {
				return -2 // Missing escaped character
			}
			if k >= len(target) {
				return 0
			}
			fallthrough
		default:
			if ignoreCase {
				if !strings.EqualFold(string(pat[i]), string(target[k])) {
					return 0
				}
			} else {
				if pat[i] != target[k] {
					return 0
				}
			}
			k++
			i++
		}
	}
	return k
}

// Create new environment, used in user procs.
func (tcl *Tcl) newEnv() *tclEnv {
	newEnv := &tclEnv{level: tcl.level}
	newEnv.vars = make(map[string]*tclVar)
	newEnv.local = make(map[string]bool)
	return newEnv
}

// Set variable in new environment.
func (tcl *Tcl) setVarNewEnv(newEnv *tclEnv, name string, value string, local bool) {
	newEnv.vars[name] = &tclVar{value: value}
	newEnv.local[name] = local
}

// Make new environment current.
func (tcl *Tcl) pushEnv(newEnv *tclEnv) {
	newEnv.parent = tcl.env
	tcl.env = newEnv
	tcl.level++
}

// Return to previous environment.
func (tcl *Tcl) popEnv() {
	tcl.env = tcl.env.parent
	tcl.level--
}

// Return pointer to environment at a given level.
func (tcl *Tcl) getLevel(top bool, level int) *tclEnv {
	if top {
		level = tcl.level - level
	}

	if level < 0 {
		return nil
	}

	callFrame := tcl.env
	for l := 0; l < level && callFrame.parent != nil; l++ {
		callFrame = callFrame.parent
	}

	return callFrame
}
