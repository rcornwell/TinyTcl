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

package tclfile

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tcl "github.com/rcornwell/tinyTCL/tcl"
)

type cases struct {
	test  string
	match string
	res   int
}

func TestFileOps(t *testing.T) {
	// Create a test file.
	tmp, err := os.MkdirTemp("/tmp", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	path := filepath.Join(tmp, "testing.txt")
	f, err := os.Create(path)
	if err != nil {
		t.Error(err.Error())
		return
	}
	name := f.Name()
	for i := range 50 {
		fmt.Fprintf(f, "%05d ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789\n", i)
	}
	f.Close()
	base := filepath.Base(name)

	testCases := []cases{
		{"file exists " + name, "1", tcl.RetOk},
		{"file size " + name, "3950", tcl.RetOk},
		{"file type " + name, "file", tcl.RetOk},
		{"file separator", string(filepath.Separator), tcl.RetOk},
		{"file dirname " + name, tmp, tcl.RetOk},
		{"file extension " + name, ".txt", tcl.RetOk},
		{"file rootname " + name, tmp, tcl.RetOk},
		{"file tail " + name, base, tcl.RetOk},
		{"file join a b /foo bar", "/foo/bar", tcl.RetOk},
		{"file join a b c", filepath.Join("a", "b", "c"), tcl.RetOk},
		{"file cwd " + tmp, "", tcl.RetOk},
		{"file cwd " + tmp + "; file pwd", tmp, tcl.RetOk},
		{"file cwd " + tmp + " ; file mkdir x; file copy " + base + " x; file cwd x; file dir", base, tcl.RetOk},
		{"file cwd " + tmp + "; file type x", "directory", tcl.RetOk},
		{"file cwd " + tmp + "; file type x/" + base, "file", tcl.RetOk},
		{"file cwd " + tmp + "; file type y", "file y does not exist", tcl.RetError},
		{"file cwd " + tmp + "; file isdirectory x", "1", tcl.RetOk},
		{"file cwd " + tmp + "; file isfile " + base, "1", tcl.RetOk},
		{"file cwd " + tmp + "; file isdirectory " + base, "0", tcl.RetOk},
		{"file cwd " + tmp + "; file isfile x", "0", tcl.RetOk},
		{"file dir " + tmp, base + " x", tcl.RetOk},
		{"file cwd " + tmp + "/x; file rename " + base + " " + base + "2 ; file dir", base + "2", tcl.RetOk},
		{"file cwd " + tmp + "/x; file delete " + base + "2 ; file exists " + base + "2", "0", tcl.RetOk},
	}

	for _, test := range testCases {
		tc := tcl.NewTCL()
		FileInit(tc)
		ret := tc.EvalString(test.test)
		switch test.res {
		case tcl.RetOk:
			if ret != nil {
				t.Errorf("Eval did not return correct results for expected: '%s' got: '%s'", test.match, ret.Error())
			}
			if test.match != tc.GetResult() {
				t.Errorf("Eval returned wrong result, got: '%s' expected: '%s'", tc.GetResult(), test.match)
			}

		case tcl.RetError:
			if ret == nil {
				t.Error("Eval did not return error as expected", test.test)
			}
		}
	}
}

func TestFileRead(t *testing.T) {
	// Create a test file.
	tmp, err := os.MkdirTemp("/tmp", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	path := filepath.Join(tmp, "testing.txt")
	f, err := os.Create(path)
	if err != nil {
		t.Error(err.Error())
		return
	}
	name := f.Name()
	for i := range 50 {
		fmt.Fprintf(f, "%05d ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789\n", i)
	}
	f.Close()

	testCases := []cases{
		{"open " + name + "; lsort [file channels] ", "file7 stderr stdin stdout", tcl.RetOk},
		{
			"set fd [open " + name + "] ; gets $fd ",
			"00000 ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
			tcl.RetOk,
		},
		{
			"set fd [open " + name + "] ; read $fd 78",
			"00000 ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
			tcl.RetOk,
		},
		{"set fd [open " + name + "] ; read $fd 78; tell $fd", "78", tcl.RetOk},
		{"set fd [open " + name + "] ; seek $fd 80; tell $fd", "80", tcl.RetOk},
		{"set fd [open " + name + "] ; seek $fd 80; seek $fd 80 current ; tell $fd", "160", tcl.RetOk},
		{
			"set fd [open " + name + "] ; seek $fd 158; gets $fd",
			"00002 ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
			tcl.RetOk,
		},
		{
			"set fd [open " + name + "] ; seek $fd -79 end; gets $fd",
			"00049 ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
			tcl.RetOk,
		},
	}

	for _, test := range testCases {
		tc := tcl.NewTCL()
		FileInit(tc)
		ret := tc.EvalString(test.test)
		switch test.res {
		case tcl.RetOk:
			if ret != nil {
				t.Errorf("Eval did not return correct results for expected: '%s' got: '%s'", test.match, ret.Error())
			}
			if test.match != tc.GetResult() {
				t.Errorf("Eval returned wrong result, got: '%s' expected: '%s'", tc.GetResult(), test.match)
			}

		case tcl.RetError:
			if ret == nil {
				t.Error("Eval did not return error as expected", test.test)
			}
		}
	}
}

func TestFileWrite(t *testing.T) {
	// Create a test file.
	tmp, err := os.MkdirTemp("/tmp", "")
	if err != nil {
		log.Fatal(err)
	}

	defer os.RemoveAll(tmp)
	name := "testing.txt"

	fileContents := []string{
		"proc Factorial {x} {",
		"   set i 1; set product 1",
		"   while {$i <= $x} {",
		"      set product [expr $product * $i]",
		"      incr i",
		"    }",
		"    return $product",
		"}",
	}

	tc := tcl.NewTCL()
	FileInit(tc)
	ret := tc.EvalString("file cwd " + tmp + "; set fd [open " + name + " w]")
	if ret != nil {
		t.Error("Unable to create file '", ret, "' ", name, "' ", tmp)
		return
	}

	text := ""
	for _, line := range fileContents {
		line = strings.ReplaceAll(line, "$", "\\$")
		line = strings.ReplaceAll(line, "[", "\\[")
		line = strings.ReplaceAll(line, "]", "\\]")
		text = "puts $fd \"" + line + "\""

		ret = tc.EvalString(text)
		if ret != nil {
			t.Error("Unable to write file ", name, "line ", text, " error ", tc.GetResult())
			return
		}
	}

	ret = tc.EvalString("close $fd")
	if ret != nil {
		t.Error("Unable to close file " + name)
		return
	}
	ret = tc.EvalString("set fd [open " + name + "]")
	if ret != nil {
		t.Error("Unable to reopen file " + name)
		return
	}
	for _, line := range fileContents {
		ret = tc.EvalString("gets $fd")
		if ret != nil {
			t.Error("Unable to read file " + name)
			return
		}
		if line != tc.GetResult() {
			t.Error("file read error got: '" + tc.GetResult() + "' expected: '" + line + "'")
		}
	}
	ret = tc.EvalString("close $fd")
	if ret != nil {
		t.Error("Unable to close file " + name)
		return
	}

	ret = tc.EvalString("source " + name)
	if ret != nil {
		t.Error("Unable to source file " + name)
		return
	}

	ret = tc.EvalString("Factorial 10")
	if ret != nil {
		t.Error("Unable to run test proc ")
		return
	}
	if tc.GetResult() != "3628800" {
		t.Error("Did not get correct results got: " + tc.GetResult())
	}

	ret = tc.EvalString("set fd [open " + name + " r]")
	if ret != nil {
		t.Error("Unable to reopen file " + name)
		return
	}

	ret = tc.EvalString("read $fd")
	if ret != nil {
		t.Error("Unable to read file " + name)
		return
	}
	if strings.Join(fileContents, "\n")+"\n" != tc.GetResult() {
		t.Error("file read error got: '" + tc.GetResult() + "' expected: '" + strings.Join(fileContents, "\n") + "'")
	}

	ret = tc.EvalString("close $fd")
	if ret != nil {
		t.Error("Unable to close file " + name)
		return
	}

	ret = tc.EvalString("set fd [open " + name + " a]")
	if ret != nil {
		t.Error("Unable to reopen file " + name)
		return
	}

	ret = tc.EvalString("puts $fd \"set result [Factorial 10]\"")
	if ret != nil {
		t.Error("Unable to append to file " + name)
		return
	}

	ret = tc.EvalString("close $fd")
	if ret != nil {
		t.Error("Unable to close file " + name)
		return
	}

	tc2 := tcl.NewTCL()
	FileInit(tc2)
	ret = tc2.EvalString("source " + name)
	if ret != nil {
		t.Error("Unable to source file " + name)
		return
	}
	if tc2.GetResult() != "" {
		t.Error("Did not get correct results got: " + tc2.GetResult())
	}
	ok, val := tc2.GetVarValue("result")
	if ok != tcl.RetOk {
		t.Error("result not set")
	}
	if val != "3628800" {
		t.Error("Did not get correct results got: " + val)
	}
}
