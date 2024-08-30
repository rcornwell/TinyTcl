/*
 * TCL  Expect matching routines.
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
	"errors"
	"io"
	"os"
	"strings"

	tcl "github.com/rcornwell/tinyTCL/tcl"
)

type matchBuffer struct {
	matchBuffer string // Current input buffer.
	Max         int    // Maximum size of buffer.
	Length      int    // Current length of buffer.
}

// Match parameters.
type matchList struct {
	pattern    string // String to match against.
	matchPos   int    // Position in match string.
	bufferPos  int    // Position in match buffer.
	exact      bool   // Exact match requested.
	glob       bool   // Glob expression.
	cmd        bool   // String is not a pattern.
	body       string // Match execution.
	ignoreCase bool   // Ignore case on match.
	echo       bool   // Echo matches out.
	nobuffer   bool   // Don't buffer matches for this pattern.
}

// Scan match list and generate list of match patterns.
func scanMatch(patterns []string, input bool) ([]*matchList, []*matchList) {
	mlin := []*matchList{}
	mlout := []*matchList{}
	match := &matchList{glob: true}
	timeout := false
	output := !input
	body := false
	flag := false
	for _, v := range patterns {
		// For interact, timeout can have optional timeout value.
		if input && timeout {
			t, _, ok := tcl.ConvertStringToNumber(v, 10, 0)
			if ok {
				match.bufferPos = t
				timeout = false
				continue
			}
		}

		// If got pattern, next should be body.
		if body {
			match.body = v
			match = &matchList{glob: true}
			body = false
			flag = false
			continue
		}

		// Not body, see if minus option.
		if !flag {
			switch v {
			case "-o":
				if !output {
					output = true
					continue
				}

			case "-ex":
				match.exact = true
				match.glob = false
				flag = true
				continue

			case "-gl":
				match.glob = true
				flag = true
				continue

			case "-echo":
				match.echo = true
				continue

			case "-nobuffer":
				match.nobuffer = true
				continue

			case "-nocase":
				match.ignoreCase = true
				continue

			default:
			}
		}

		// If not exact match see if special pattern.
		if !match.exact {
			switch v {
			case "timeout":
				if input {
					timeout = true
				}
				match.cmd = true
			case "eof", "default", "full_buffer":
				match.cmd = true
			}
		}

		// Put new pattern into correct list.
		if match.ignoreCase {
			v = strings.ToLower(v)
		}
		match.pattern = v
		if output {
			mlout = append(mlout, match)
		} else {
			mlin = append(mlin, match)
		}
		body = true
	}

	return mlin, mlout
}

// Append a bunch of text to a input buffer.
func appendMatch(ml []*matchList, mbuf *matchBuffer, by []byte) {
	mbuf.matchBuffer += string(by)
	mbuf.Length += len(by)
	if mbuf.Length < mbuf.Max {
		return
	}

	// Back up match pointers.
	shift := mbuf.Length - mbuf.Max
	mbuf.matchBuffer = mbuf.matchBuffer[shift:]
	mbuf.Length = len(mbuf.matchBuffer)
	for i := range len(ml) {
		ml[i].bufferPos = max(0, ml[i].bufferPos-shift)
		ml[i].matchPos = max(0, ml[i].matchPos-shift)
	}
}

// When we get a match shift input buffer to position.
func moveBuffer(ml []*matchList, mbuf *matchBuffer, pos int) {
	mbuf.matchBuffer = mbuf.matchBuffer[pos:]
	mbuf.Length = len(mbuf.matchBuffer)
	for j := range len(ml) {
		ml[j].bufferPos = 0
		ml[j].matchPos = 0
	}
}

// Attempt to find a match in current buffer.
func match(t *tcl.Tcl, ml []*matchList, mbuf *matchBuffer) (int, bool) {
	m := false
	did := true
	// Continue to loop while there was at least one match.
	for did {
		did = false

		// Check each pattern.
		for i := range ml {
			// If this is command, or match position past end of buffer,
			// skip this pattern.
			if ml[i].cmd || ml[i].matchPos > mbuf.Length || mbuf.Length == 0 {
				continue
			}

			// If glob match, scan full string to see if pattern in it.
			if ml[i].glob {
				for j := range mbuf.Length - 1 {
					matchString := mbuf.matchBuffer[j:]
					r := tcl.Match(ml[i].pattern, matchString, ml[i].ignoreCase, len(matchString))
					if r > 0 {
						moveBuffer(ml, mbuf, j+r)
						if ml[i].body == "" {
							return ExpEnd, true
						}
						return t.Eval(ml[i].body), true
					}
				}
				continue
			}
			did = true
			by := mbuf.matchBuffer[ml[i].matchPos]
			if ml[i].ignoreCase {
				by = strings.ToLower(string(by))[0]
			}
			ml[i].matchPos++
			match := ml[i].pattern[ml[i].bufferPos]
			if match == by {
				ml[i].bufferPos++
				if ml[i].bufferPos == len(ml[i].pattern) {
					moveBuffer(ml, mbuf, ml[i].matchPos)
					if ml[i].body == "" {
						return ExpEnd, true
					}
					return t.Eval(ml[i].body), true
				}
				if ml[i].echo {
					os.Stdout.Write([]byte{by})
				}
				m = true
			} else {
				ml[i].bufferPos = 0
			}
		}
	}
	return -1, m
}

// Match special patterns.
func matchSpecial(t *tcl.Tcl, ml []*matchList, str string) int {
	for i := range ml {
		if ml[i].cmd {
			if ml[i].pattern == str || ml[i].pattern == "default" {
				if ml[i].body != "" {
					return t.Eval(ml[i].body)
				}
				break
			}
		}
	}

	return ExpEnd
}

// Find any timeout values in match list.
func getTimeout(ml []*matchList) int {
	for i := range ml {
		if ml[i].cmd && ml[i].pattern == "timeout" {
			return ml[i].bufferPos
		}
	}
	return -1
}

// Process input from remote system.
func processRemote(proc *expectProcess, input []byte, err error) int {
	if errors.Is(err, io.EOF) {
		return matchSpecial(proc.tcl, proc.matchPats, "eof")
	}

	if err != nil {
		return tcl.RetError
	}

	if proc.matching {
		appendMatch(proc.matchPats, &proc.matchData, input)
		r, _ := match(proc.tcl, proc.matchPats, &proc.matchData)
		return r
	}

	proc.last = append(proc.last, input...)
	return tcl.RetOk
}
