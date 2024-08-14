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
	"fmt"
	"os"

	tcl "github.com/rcornwell/tinyTCL/tcl"
)

type matchBuffer struct {
	matchBuffer string // Current input buffer.
	Max         int    // Maximum size of buffer.
	Length      int    // Current length of buffer.
}

// Match parameters.
type matchList struct {
	str      string // String to match against.
	mpos     int    // Position in match string.
	bpos     int    // Position in match buffer.
	exact    bool   // Exact match requested.
	glob     bool   // Glob expression.
	cmd      bool   // String is not a pattern.
	body     string // Match execution.
	nocase   bool   // Ignore case on match.
	echo     bool   // Echo matches out.
	nobuffer bool   // Don't buffer matches for this pattern.
}

//  func cmdExpect(t *tcl.Tcl, _ []string) int {
// 	 ok, spawnID := t.GetVarValue("spawn_id")
// 	 if ok != tcl.RetOk {
// 		 return t.SetResult(tcl.RetError, "spawn_id variable not defined")
// 	 }
// 	 expect, eok := t.Data["expect"].(*expectData)
// 	 if !eok {
// 		 panic("invalid data type expect extension")
// 	 }

// 	 proc, sok := expect.processes[spawnID]
// 	 if !sok {
// 		 return t.SetResult(tcl.RetError, "no process of name "+spawnID)
// 	 }

// 	 proc.matching = true

// 	 proc.rdr.startReader(nil)
// 	 proc.matching = true
// 	 defer func() { proc.matching = false }()

// 	 // ml := []matchList{}
// 	 // patterns := []string{}
// 	 // // Build match patterns.
// 	 // if len(args) == 2 {
// 	 // 	patterns = tcl.NewTCL().ParseArgs(args[1])
// 	 // } else {
// 	 // 	patterns = args[1:]
// 	 // }
// 	 defer proc.rdr.stopReader()
// 	 for {
// 		 //	by, err := stdin.read()
// 		 input := make([]byte, 1)
// 		 _, err := os.Stdin.Read(input)
// 		 if err != nil {
// 			 return t.SetResult(tcl.RetError, "expect read "+err.Error())
// 		 }
// 		 if input[0] == '\001' {
// 			 break
// 		 }
// 		 //	proc.pty.Write(input)
// 		 // _, err = proc.stdin.Write([]byte{by})
// 		 //	_, err = os.Stdout.Write([]byte{by})
// 		 // if err != nil {
// 		 // 	stdin.stopReader()
// 		 // 	return t.SetResult(tcl.RetError, "write: "+err.Error())
// 		 // }
// 	 }
// 	 return t.SetResult(tcl.RetOk, "")
//  }

func scanMatch(patterns []string, input bool) ([]*matchList, []*matchList) {
	mlin := []*matchList{}
	mlout := []*matchList{}
	match := &matchList{}
	output := !input
	body := false
	fmt.Println(patterns)
	for _, v := range patterns {
		if body {
			match.body = v
			match = &matchList{}
			body = false
			continue
		}

		if !match.exact {
			switch v {
			case "-o":
				if !output {
					output = true
					continue
				}

			case "-ex":
				match.exact = true
				continue

			case "-gl":
				match.glob = true
				continue

			case "-echo":
				match.echo = true
				continue

			case "-nobuffer":
				match.nobuffer = true
				continue

			case "-nocase":
				match.nocase = true
				continue

			case "-timeout":
			default:
			}
		}

		if !match.exact {
			switch v {
			case "timeout", "eof", "default":
				match.str = v
				match.cmd = true
				body = true
				continue
			}
		}

		//	fmt.Printf(" match %02x %s\n", v[0], v)
		match.str = v
		if output {
			mlout = append(mlout, match)
		} else {
			mlin = append(mlin, match)
		}
		body = true
	}

	return mlin, mlout
}

// 	 proc.matchPats = mlout
// 	 proc.matching = true
// 	 proc.matchData.matchBuffer = ""
// 	 proc.matchData.matchLen = -1
// 	 defer func() { proc.matching = false }()
// 	 mbuf := matchBuffer{matchLen: -1, matchMax: 2000}

// 	 if proc.last != 0 {
// 		 ret, _ := process(proc, proc.last, nil)
// 		 if ret != tcl.RetOk {
// 			 return t.SetResult(ret, "")
// 		 }
// 		 proc.last = 0
// 	 }

// 	 proc.rdr.startReader(os.Stdin)
// 	 defer proc.rdr.stopReader()
// 	 for {
// 		 ret, by, err := proc.rdr.read()
// 		 if err != nil {
// 			 return t.SetResult(tcl.RetError, "read "+err.Error())
// 		 }
// 		 if ret >= 0 {
// 			 switch ret {
// 			 case tcl.RetError:
// 				 return t.SetResult(tcl.RetError, "read error")
// 			 case tcl.RetBreak:
// 			 case tcl.RetReturn:
// 			 case tcl.RetContinue:
// 			 }
// 			 continue
// 		 }

// 		 fmt.Println("read: " + string(by))
// 		 appendMatch(mlin, &mbuf, by)
// 		 r, m := match(t, mlin, &mbuf)
// 		 switch r {
// 		 case tcl.RetOk:
// 		 case tcl.RetError, tcl.RetBreak, tcl.RetContinue, tcl.RetReturn:
// 			 return t.SetResult(r, "")
// 		 }
// 		 if by == '\001' {
// 			 break
// 		 }
// 		 if !m {
// 			 _, err = proc.pty.Write([]byte{by})
// 		 }
// 		 //_, err = proc.stdin.Write([]byte{by})
// 		 //	_, err = os.Stdout.Write([]byte{by})
// 		 if err != nil {
// 			 return t.SetResult(tcl.RetError, "write: "+err.Error())
// 		 }
// 	 }
// 	 return t.SetResult(tcl.RetOk, "")
//  }

func appendMatch(ml []*matchList, mbuf *matchBuffer, by []byte) {
	mbuf.matchBuffer += string(by)
	mbuf.Length += len(by)
	fmt.Printf("append '%s' %d %d\n", string(by), mbuf.Max, mbuf.Length)
	if mbuf.Length < mbuf.Max {
		return
	}

	// Back up match pointers.
	shift := mbuf.Length - mbuf.Max
	fmt.Printf("Shift buffer '%s' %d %d %d\n", string(mbuf.matchBuffer), mbuf.Max, mbuf.Length, shift)
	mbuf.matchBuffer = mbuf.matchBuffer[shift:]
	mbuf.Length = len(mbuf.matchBuffer)
	for i := range len(ml) {
		ml[i].bpos = max(0, ml[i].bpos-shift)
		ml[i].mpos = max(0, ml[i].mpos-shift)
	}
}

func moveBuffer(ml []*matchList, mbuf *matchBuffer, pos int) {
	mbuf.matchBuffer = mbuf.matchBuffer[pos:]
	mbuf.Length = len(mbuf.matchBuffer)
	for j := range len(ml) {
		ml[j].bpos = 0
		ml[j].mpos = 0
	}
}

func match(t *tcl.Tcl, ml []*matchList, mbuf *matchBuffer) (int, bool) {
	//	fmt.Println("match " + mbuf.matchBuffer)
	// Character to match against.

	m := false
	did := true
	for did {
		did = false

		for i := range ml {
			if ml[i].cmd {
				continue
			}
			if ml[i].mpos >= mbuf.Length {
				//			fmt.Printf("Skip %d %d %d\n", ml[i].mpos, mbuf.Length, i)
				continue
			}
			did = true
			by := mbuf.matchBuffer[ml[i].mpos]
			ml[i].mpos++
			match := ml[i].str[ml[i].bpos]
			//		fmt.Printf("check %d %d %d %d %02x %02x\n", ml[i].mpos, ml[i].bpos, mbuf.Length, i, by, match)
			if match == by {
				ml[i].bpos++
				if ml[i].bpos == len(ml[i].str) {
					//		fmt.Println("Match " + ml[i].str)
					moveBuffer(ml, mbuf, ml[i].mpos)
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
				ml[i].bpos = 0
			}
		}
	}
	return -1, m
}

func process(proc *expectProcess, input []byte, err error) (int, bool) {
	if err != nil {
		// treat as end of file.
		return tcl.RetExit, true
	}
	if proc.matching {
		appendMatch(proc.matchPats, &proc.matchData, input)
		r, _ := match(proc.tcl, proc.matchPats, &proc.matchData)
		return r, false
	}
	proc.last = append(proc.last, input...)
	return tcl.RetOk, false
}
