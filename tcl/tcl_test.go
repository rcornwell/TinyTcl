/*
 * TCL  Test set for TCL.
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
	"testing"
)

type cases struct {
	test  string
	match string
	res   int
}

func TestUnescape(t *testing.T) {
	testCases := []cases{
		{"", "", 0},
		{"a", "a", 1},
		{"\\t", "\t", 1},
		{"\\ta", "\ta", 2},
		{"a\\[", "a[", 2},
		{"a\\[\\[", "a[[", 3},
		{"a\\[z\\[a", "a[z[a", 5},
		{"\\\\", "\\", 1},
		{"\\x30", "0", 1},
		{"\\xZ", "", -2},
		{"\\xZZ", "", -2},
		{"\\x9", "\x09", 1},
		{"\\x9Z", "\x09Z", 2},
		{"\\x300", "00", 2},
		{"\\x310", "10", 2},
		{"\\x31\\x312", "112", 3},
		{"x\\x31\\x312", "x112", 4},
	}

	for _, test := range testCases {
		res, n := UnEscape(test.test)
		if res != test.match {
			t.Errorf("String %s did not match results %s", test.test, res)
		}
		if test.res == -2 {
			if n != -2 {
				t.Errorf("String %s did not error out", test.test)
			}
		} else {
			if test.res != len(res) {
				t.Errorf("String %s not correct length: %d %d", test.test, test.res, len(res))
			}
		}
	}
}

func TestGetVars(t *testing.T) {
	testCases := []cases{
		{"a", "54", RetOk},
		{"b", "3", RetOk},
		{"c", "-4x", RetOk},
		{"d", "", RetError},
	}
	tcl := NewTCL()
	r := tcl.eval("set a 54; set b 3; set c -4x", parserOptions{})
	if r != RetOk {
		t.Error("Eval did not return OK")
		return
	}
	for _, test := range testCases {
		n, v := tcl.GetVarValue(test.test)
		if n != test.res {
			t.Errorf("Did not set variable " + test.test)
		}
		if test.res == RetOk && test.match != v {
			t.Error("Variable: " + test.match + " Did not get correct value")
		}
	}
	tcl.SetVarValue("d", "123")
	n, v := tcl.GetVarValue("d")
	if n != RetOk {
		t.Errorf("Did not set variable d")
	}
	if v != "123" {
		t.Error("Variable: d  Did not get correct value")
	}
}

func TestEval(t *testing.T) {
	testCases := []cases{
		{"expr 1 + 2", "3", RetOk},
		{"expr 2*4", "8", RetOk},
		{"expr -2", "-2", RetOk},
		{"expr - 2", "-2", RetOk},
		{"expr = 2", "invalid operator", RetError},
		{"set x \"$\"", "$", RetOk},
		{"set x \"val$\"", "val$", RetOk},
		{"set x \"${}\"", "${}", RetOk},
		{"set x \"${ }\"", "${ }", RetOk},
		{"set x \"${\"", "${", RetOk},
		{"proc foo {a} {set v $a}; foo b", "b", RetOk},
		{"proc foo {} {set v a}; foo", "a", RetOk},
		{"set var 0 ;for {set i 1} {$i<=10} {incr i} { append var \",\" $i}; set var", "0,1,2,3,4,5,6,7,8,9,10", RetOk},
		{"break", "", RetBreak},
		{"set y {}; for {set x 0} {$x<10} {incr x} { if {$x > 5} { break } ;append y \",$x\" }; set y", ",0,1,2,3,4,5", RetOk},
		{"set y {}; for {set x 0} {$x<10} {incr x} { if {$x == 5} { continue } ;append y \",$x\" }; set y", ",0,1,2,3,4,6,7,8,9", RetOk},
		{"proc foo {} {catch {expr {1 +- }}}; foo", "1", RetOk},
		{"set x 0;while {$x<10} { incr x} ; set x", "10", RetOk},
		{"concat a b {c d e} {f {g h}}", "a b c d e f {g h}", RetOk},
		{"concat \" a b {c   \" d \"  e} f\"", "a b {c d e} f", RetOk},
		{"concat \"a   b   c\" { d e f }", "a   b   c d e f", RetOk},
		{"if {1+2 != 3} { error \"something is very wrong with addition\"}", "something is very wrong with addition", RetError},
		{"set data {1 2 3 4 5};join $data \", \"", "1, 2, 3, 4, 5", RetOk},
		{"set data {1 {2 3} 4 {5 {6 7} 8}}; join $data", "1 2 3 4 5 {6 7} 8", RetOk},
		{"set var 1; lappend var 2", "1 2", RetOk},
		{"set var 1; lappend var 2; lappend var 3 4 5", "1 2 3 4 5", RetOk},
		{"set var {}; lappend x 1 2 3; set x", "1 2 3", RetOk},
		{"lindex {a b c}", "a b c", RetOk},
		{"lindex {a b c} {}", "a b c", RetOk},
		{"lindex {a b c} 0", "a", RetOk},
		{"lindex {a b c} 2", "c", RetOk},
		{"lindex {a b c} end", "c", RetOk},
		{"lindex {a b c} end-1", "b", RetOk},
		{"lindex {{a b c} {d e f} {g h i}} 2 1", "h", RetOk},
		{"lindex {{a b c} {d e f} {g h i}} {2 1}", "h", RetOk},
		{"lindex {{{a b} {c d}} {{e f} {g h}}} 1 1 0", "g", RetOk},
		{"lindex {{{a b} {c d}} {{e f} {g h}}} {1 1 0}", "g", RetOk},
		{"set var {some {elements to} select}; lindex $var 1", "elements to", RetOk},
		{
			"set oldList {the fox jumps over the dog}; set midList [linsert $oldList 1 quick]",
			"the quick fox jumps over the dog", RetOk,
		},
		{
			"set oldList {the fox jumps over the dog}; set midList [linsert $oldList 1 quick]; set newList [linsert $midList end-1 lazy]",
			"the quick fox jumps over the lazy dog", RetOk,
		},
		{
			"set oldList {the fox jumps over the dog}; set newerList [linsert [linsert $oldList end-1 quick] 1 lazy]",
			"the lazy fox jumps over the quick dog", RetOk,
		},
		{"list a b \"c d e  \" \"  f {g h}\"", "a b {c d e  } {  f {g h}}", RetOk},
		{"llength {a b c d e}", "5", RetOk},
		{"llength {a b c}", "3", RetOk},
		{"llength {}", "0", RetOk},
		{"llength {a b {c d} e}", "4", RetOk},
		{"llength {a b { } c d e}", "6", RetOk},
		{"set var { }; set x \"[string length $var],[llength $var]\"", "1,0", RetOk},
		{"lrange {a b c d e} 0 1", "a b", RetOk},
		{"lrange {a b c d e} end-2 end", "c d e", RetOk},
		{"lrange {a b c d e} 1 end-1", "b c d", RetOk},
		{"set var {some {elements to} select};lrange $var 1 1", "{elements to}", RetOk},
		{"lreplace {a b c d e} 1 1 foo", "a foo c d e", RetOk},
		{"lreplace {a b c d e} 1 2 three more elements", "a three more elements d e", RetOk},
		{"set var {a b c d e}\n set var [lreplace $var end end]", "a b c d", RetOk},
		{"lsearch {a b c d e} c", "2", RetOk},
		{"lsearch -all {a b c a b c} c", "2 5", RetOk},
		{"lsearch -inline {a20 b35 c47} b*", "b35", RetOk},
		{"lsearch -inline -not {a20 b35 c47} b*", "a20", RetOk},
		{"lsearch -all -inline -not {a20 b35 c47} b*", "a20 c47", RetOk},
		{"lsearch -all -not {a20 b35 c47} b*", "0 2", RetOk},
		{"lsearch -start 3 {a b c a b c} c", "5", RetOk},
		{"set x [list [list a b c] [list d e f] [list g h i]];lset x {j k l}", "j k l", RetOk},
		{"set x [list [list a b c] [list d e f] [list g h i]];lset x {} {j k l}", "j k l", RetOk},
		{"set x [list [list a b c] [list d e f] [list g h i]];lset x 0 j", "j {d e f} {g h i}", RetOk},
		{"set x [list [list a b c] [list d e f] [list g h i]];lset x 2 j", "{a b c} {d e f} j", RetOk},
		{"set x [list [list a b c] [list d e f] [list g h i]];lset x end j", "{a b c} {d e f} j", RetOk},
		{"set x [list [list a b c] [list d e f] [list g h i]];lset x end-1 j", "{a b c} j {g h i}", RetOk},
		{"set x [list [list a b c] [list d e f] [list g h i]];lset x 2 1 j", "{a b c} {d e f} {g j i}", RetOk},
		{"set x [list [list a b c] [list d e f] [list g h i]];lset x {2 1} j", "{a b c} {d e f} {g j i}", RetOk},
		{"set x [list [list a b c] [list d e f] [list g h i]];lset x {2 3} j ", "list index out of range", RetError},
		{"set x [list [list [list a b] [list c d]] \\\n    [list [list e f] [list g h]]]", "{{a b} {c d}} {{e f} {g h}}", RetOk},
		{"set x [list [list [list a b] [list c d]] [list [list e f] [list g h]]]; lset x 1 1 0 j", "{{a b} {c d}} {{e f} {j h}}", RetOk},
		{
			"set x [list [list [list a b] [list c d]] [list [list e f] [list g h]]]; lset x {1 1 0} j",
			"{{a b} {c d}} {{e f} {j h}}", RetOk,
		},
		{"lsort {a10 B2 b1 a1 a2}", "B2 a1 a10 a2 b1", RetOk},
		{"lsort {{a 5} { c 3} {b 4} {e 1} {d 2}}", "{ c 3} {a 5} {b 4} {d 2} {e 1}", RetOk},
		{"lsort -integer {5 3 1 2 11 4}", "1 2 3 4 5 11", RetOk},
		{"lsort -integer {1 2 0x5 7 0 4 -1}", "-1 0 1 2 4 0x5 7", RetOk},
		{"split \"comp.lang.tcl.announce\" .", "comp lang tcl announce", RetOk},
		{"split \"alpha beta gamma\" \"temp\"", "al {ha b} {} {a ga} {} a", RetOk},
		{"split \"Example with {unbalanced brace character\"", "Example with \\{unbalanced brace character", RetOk},
		{"split \"Hello world\" {}", "H e l l o { } w o r l d", RetOk},
		{"set a 44; subst {xyz {$a}}", "xyz {44}", RetOk},
		{"set a \"p\\} q \\{r\"; subst {xyz {$a}}", "xyz {p} q {r}", RetOk},
		{"set a 44; subst -novariables {$a [set b $a]}", "$a 44", RetOk},
		{"subst {abc,[break],def}", "abc,", RetOk},
		{"subst {abc,[continue;expr 1+2],def}", "abc,,def", RetOk},
		{"subst {abc,[return foo;expr 1+2],def}", "abc,foo,def", RetOk},
		{"set foo \"abc\";switch abc a - b {expr 1} $foo {expr 2} default {expr 3}", "2", RetOk},
		{"switch -glob aaab {  a*b     -  b       {expr 1}   a*      {expr 2}   default {expr 3}}", "1", RetOk},
		{"switch -glob aaab { \n  a*b     -\n  b       {expr 1} \n  a*      {expr 2} \n  default {expr 3}\n}", "1", RetOk},
		{"switch xyz {  a  -   b { expr 1  }\n   c { expr 2 }\n   default { expr 3  }\n}", "3", RetOk},
		{"set x 0; while {$x<10} {    incr x }; set x", "10", RetOk},
		{"set x 0;\nwhile {$x<10} \n {\n    incr x \n};\n set x", "10", RetOk},
		{"proc accum {string} { global acc; append acc $string}; accum test; accum second;set acc", "variable acc not found", RetError},
		{"set acc {}; proc accum {string} { global acc; append acc $string}; accum test; accum ,second;set acc", "test,second", RetOk},
		{"set test 5;proc add2 name {upvar $name x; set x [expr $x+2]}; add2 test; set test", "7", RetOk},
		{"proc a {value} {set x 6; b $value}; proc b name { upvar 2 $name z k y; set z 4; set y 3}; set k 0; set v 10;" +
			" set x 1; a x; set x", "4", RetOk},
		{"variable x 5; set x", "5", RetOk},
		{"variable a 1 b 2; set x \"$a $b\"", "1 2", RetOk},
		{"set x 5; unset x; set x", "value: x not found", RetError},
		{"proc a {value} {set x 6; b $value}; proc b name { set z 4; set y 3}; set k 0; set v 10; set x 1; a x; set x", "1", RetOk},
		{"string first a 0a23456789abcdef 5", "10", RetOk},
		{"string first a 0a23456789abcdef 11", "-1", RetOk},
		{"string last a 0a23456789abcdef 15", "10", RetOk},
		{"string last a 0a23456789abcdef 9", "1", RetOk},
		{"string first abc 0a23456789abcdef 5", "10", RetOk},
		{"string first abc 0a23456789abcdef 11", "-1", RetOk},
		{"string last abc 0a23456789abcdef 15", "10", RetOk},
		{"string last abc 0a23456789abcdef 9", "-1", RetOk},
		{"string map {abc 1 ab 2 a 3 1 0} 1abcaababcabababc", "01321221", RetOk},
		{"string map {abc 1 ab 2 a 3 1 0} 1abcaababcefabababc", "01321ef221", RetOk},
		{"string map {1 0 ab 2 a 3 abc 1} 1abcaababcabababc", "02c322c222c", RetOk},
		{"string totitle \"hello world\"", "Hello world", RetOk},
		{"string toupper \"hello world\"", "HELLO WORLD", RetOk},
		{"string toupper \"hello world\" 5 8", "hello WORld", RetOk},
		{"string tolower \"HeLlo World\"", "hello world", RetOk},
		{"string is alpha \"hello\"", "1", RetOk},
		{"string is alpha \"helo8]\"", "0", RetOk},
		{"string is alpha -failindex x \"hello8\" ; set x", "5", RetOk},
		{"string is alpha -failindex x \"hello\"; info exists x", "0", RetOk},
		{"string range \"abcde\" 0 3", "abcd", RetOk},
		{"string range \"abcdefgh\" 3 5", "def", RetOk},
		{"string range \"abcdefgh\" 5 3", "", RetOk},
		{"string index \"abcde\" 3", "d", RetOk},
		{"string index \"abcde\" end-2", "c", RetOk},
		{"string index \"abcde\" 10", "", RetOk},
		{"string match \"fred*\" \"freda\"", "1", RetOk},
		{"string equal \"fred*\" \"freda\"", "0", RetOk},
		{"string equal -nocase -length 3 \"abcde\" \"abcdefg\"", "1", RetOk},
		{"string equal -length 0 a b", "1", RetOk},
		{"string replace \"this is a bad example\" 10 12 good", "this is a good example", RetOk},
		{"string hello", "string unknown function", RetError},
		{"string repeat \"abc\" 3", "abcabcabc", RetOk},
		{"string trim \"    h e l o    \"", "h e l o", RetOk},
		{"string trimright \"    h e l o    \"", "    h e l o", RetOk},
		{"string trimleft \"    h e l o    \"", "h e l o    ", RetOk},
		{"set x {}; foreach {i j} {a b c d e f} { lappend x $j $i} ; set x", "b a d c f e", RetOk},
		{"set x {}; foreach i {a b c} j {d e f g} { lappend x $i $j}; set x", "a d b e c f {} g", RetOk},
		{"set x {}; foreach i {a b c} {j k} {d e f g} { lappend x $i $j $k}; set x", "a d e b f g c {} {}", RetOk},
		{
			"proc compare {a b} { set a0 [lindex $a 0]; set b0 [lindex $b 0]; if {$a0 < $b0} { return -1 } " +
				"elseif {$a0 > $b0} { return 1 }; return [string compare [lindex $a 1] [lindex $b 1]]}; " +
				"lsort -command compare {{3 apple} {0x2 carrot} {1 dingo} {2 banana}} {1 dingo} {2 banana} {0x2 carrot} {3 apple}",
			"{1 dingo} {2 banana} {0x2 carrot} {3 apple}",
			RetOk,
		},
		{"set x 1; set y 2; set z 3; set a {}; if {$x==1} {set a $x}; set a", "1", RetOk},
		{"set x 1; set y 2; set z 3; set a {};if {$x==1} {set a $x} else {set a $y}; set a", "1", RetOk},
		{"set x 1; set y 2; set z 3; set a {};if {$x!=1} {set a $x} else {set a $y}; set a", "2", RetOk},
		{"set x 1; set y 2; set z 3; set a {};if {$x!=1} {set a $x} elseif {$y==2} {set a $y}; set a", "2", RetOk},
		{"set x 1; set y 2; set z 3;set a {};if {$x!=1} {set a $x} elseif {$y!=2} {set a $y} else {set a $z}; set a", "3", RetOk},
		{"set x 1; incr x; rename incr add1; add1 x ; set x", "3", RetOk},
		{"set x 1; incr x; set x", "2", RetOk},
		{"set x 1; incr x 10 ; set x", "11", RetOk},
		{"proc v {var} { upvar $var v; if [catch {set v}] {return 0} else {return 1}}; v x", "0", RetOk},
		{"proc v {var} { upvar $var v; if [catch {set v}] {return 0} else {return 1}}; set x 1; v x", "1", RetOk},
		{"proc foo {} {error bogus }; catch foo result", "1", RetOk},
		{"proc foo {} {error bogus }; catch foo result; set result", "bogus", RetOk},
		{"#comment", "", RetOk},
		{"set x 5; set z 10; #comment ; set x", "10", RetOk},
		{"set x 5; set z 10; #comment \n set x", "5", RetOk},
		{"set x 5; set z 10; #comment \\\n continue \n set x", "5", RetOk},
		{"set x \"ab\\tcd\"", "ab\tcd", RetOk},
	}

	for _, test := range testCases {
		tcl := NewTCL()
		//	t.Log("Test: " + test.test)
		ret := tcl.eval(test.test, parserOptions{})
		if test.res != ret {
			t.Errorf("Eval did not return correct results for %s, got: %d, expected %d", test.test, ret, test.res)
		}
		if test.match != tcl.GetResult() {
			t.Errorf("Eval %s returned wrong result, got: '%s' expected: '%s'", test.test, tcl.GetResult(), test.match)
		}
	}
}
