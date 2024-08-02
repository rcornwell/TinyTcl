/*
 * TCL  file command.
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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	tcl "github.com/rcornwell/tinyTCL/tcl"
)

var funmap = map[string]func(*tcl.Tcl, []string) int{
	"atime":       fileType,      // name
	"channels":    fileChannels,  // ?pattern
	"copy":        fileCopy,      //  -force -- source target
	"cwd":         fileCwd,       // dir
	"delete":      fileDelete,    //  -force -- pathname???
	"dir":         fileDir,       // ?dir
	"dirname":     filePath,      // name
	"executable":  fileType,      // name
	"exists":      fileType,      // name
	"extension":   filePath,      // name
	"isdirectory": fileType,      // name
	"isfile":      fileType,      // name
	"join":        fileJoin,      // name name?
	"mkdir":       fileMkdir,     // dir?
	"readable":    fileAccess,    // name
	"rename":      fileRename,    // -force -- source target
	"rootname":    filePath,      // name
	"pwd":         filePwd,       //
	"separator":   fileSeparator, //
	"size":        fileType,      // name
	"split":       filePath,      // name
	"tail":        filePath,      // name
	"type":        fileType,      // name
	"writable":    fileAccess,    // name
}

var openModes = map[string]int{
	"r":      os.O_RDONLY,
	"r+":     os.O_RDWR | os.O_CREATE,
	"w":      os.O_WRONLY | os.O_TRUNC | os.O_CREATE,
	"w+":     os.O_RDWR | os.O_TRUNC | os.O_CREATE,
	"a":      os.O_WRONLY | os.O_APPEND | os.O_CREATE,
	"a+":     os.O_RDWR | os.O_APPEND | os.O_CREATE,
	"RDONLY": os.O_RDONLY,
	"WRONLY": os.O_WRONLY,
	"RDWR":   os.O_RDWR,
	"APPEND": os.O_APPEND,
	"CREAT":  os.O_CREATE,
	"TRUNC":  os.O_TRUNC,
}

// Handle File commands.
func cmdFile(t *tcl.Tcl, args []string, _ []string) int {
	if len(args) < 2 {
		return t.SetResult(tcl.RetError, "file function")
	}
	fn, ok := funmap[args[1]]
	if !ok {
		return t.SetResult(tcl.RetError, "file unknown function")
	}
	return fn(t, args)
}

// Return list of open channels matching optional pattern.
func fileChannels(t *tcl.Tcl, args []string) int { // ?pattern
	res := []string{}
	if len(args) > 3 {
		return t.SetResult(tcl.RetError, "file channels ?pattern")
	}

	fil, ok := t.Data["file"].(*tclFileData)
	if !ok {
		panic("invalid data type file extension")
	}

	for ch := range fil.channels {
		if len(args) > 2 && tcl.Match(args[2], ch, false, len(ch)) != 1 {
			continue
		}
		res = append(res, ch)
	}

	return t.SetResult(tcl.RetOk, strings.Join(res, " "))
}

// Copy one file to another.
func fileCopy(t *tcl.Tcl, args []string) int { //  -force -- source target
	if len(args) < 2 {
		return t.SetResult(tcl.RetError, "file copy ?-force file ?file ?target")
	}

	force := false

	i := 2
	if args[i] == "-force" {
		force = true
		i++
	}

	if len(args) < (i + 2) {
		return t.SetResult(tcl.RetError, "file copy ?-force file ?file ?target")
	}

	// Check if last argument is a directory.
	target := args[len(args)-1]
	dir := false

	stat, err := os.Stat(target)
	if err == nil {
		if stat.IsDir() {
			dir = true
		}

		if stat.Mode().IsRegular() && !force {
			return t.SetResult(tcl.RetError, "file exists and is not direcory")
		}
	}

	for len(args) > (i + 1) {
		fmt.Println("copy ", target, len(args), i, args[i])
		err = copyFile(args[i], target, dir, force)
		if err != nil {
			return t.SetResult(tcl.RetError, err.Error())
		}
		i++
	}
	return t.SetResult(tcl.RetOk, "")
}

// Copy a file to file or directory.
func copyFile(src string, dst string, dir bool, force bool) error {
	source, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !source.Mode().IsRegular() {
		return errors.New(src + "is not regular file")
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	if dir {
		dst += string(os.PathSeparator) + filepath.Base(src)
	}

	_, err = os.Stat(dst)
	if err == nil && !force {
		return errors.New("file " + dst + " exists")
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// Delete a file.
func fileDelete(t *tcl.Tcl, args []string) int { //  -force -- pathname???
	if len(args) < 2 {
		return t.SetResult(tcl.RetError, "file delete ?-force file ?file")
	}

	i := 2
	if args[i] == "-force" {
		i++
	}

	for len(args) > i {
		fmt.Println("delete ", len(args), i, args[i])
		err := os.Remove(args[i])
		if err != nil {
			return t.SetResult(tcl.RetError, err.Error())
		}
		i++
	}
	return t.SetResult(tcl.RetOk, "")
}

// Return directory name of a file name.
func filePath(t *tcl.Tcl, args []string) int { // name
	if len(args) > 3 {
		return t.SetResult(tcl.RetError, "file "+args[1]+" name")
	}
	switch args[1] {
	case "dirname": // Return directory part of a name.
		return t.SetResult(tcl.RetOk, filepath.Dir(args[2]))
	case "extension": // Return extension part of name (after last dot)
		return t.SetResult(tcl.RetOk, filepath.Ext(args[2]))
	case "rootname":
		return t.SetResult(tcl.RetOk, filepath.Dir(args[2]))
	case "split":
		return t.SetResult(tcl.RetOk, strings.Join(filepath.SplitList(args[2]), " "))
	case "tail":
		return t.SetResult(tcl.RetOk, filepath.Base(args[2]))
	}
	return t.SetResult(tcl.RetError, "Not  implemented")
}

// Returns 1 if file is of requested type, 0 if not.
func fileType(t *tcl.Tcl, args []string) int { // name
	if len(args) > 3 {
		return t.SetResult(tcl.RetError, "file "+args[1]+" name")
	}
	exists := true // Assume file exists.
	info, err := os.Lstat(args[2])
	if err != nil {
		if os.IsNotExist(err) {
			exists = false
		} else {
			return t.SetResult(tcl.RetError, err.Error())
		}
	}
	switch args[1] {
	case "atime", "mtime":
		time := info.ModTime()
		return t.SetResult(tcl.RetOk, tcl.ConvertNumberToString(int(time.Unix()), 10))

	case "exists":
		if exists {
			return t.SetResult(tcl.RetOk, "1")
		}

	case "isdirectory":
		if exists && info.IsDir() {
			return t.SetResult(tcl.RetOk, "1")
		}

	case "isfile":
		if exists && info.Mode().IsRegular() {
			return t.SetResult(tcl.RetOk, "1")
		}

	case "size":
		if exists {
			return t.SetResult(tcl.RetOk, tcl.ConvertNumberToString(int(info.Size()), 10))
		}

	case "type":
		ftype := ""
		switch mode := info.Mode(); {
		case mode.IsRegular():
			ftype = "file"
		case mode.IsDir():
			ftype = "directory"
		case mode&fs.ModeCharDevice != 0:
			ftype = "characterSpecial"
		case mode&fs.ModeDevice != 0:
			ftype = "blockSpecial"
		case mode&fs.ModeNamedPipe != 0:
			ftype = "fifo"
		case mode&fs.ModeSymlink != 0:
			ftype = "link"
		}
		return t.SetResult(tcl.RetOk, ftype)

	case "executable":
		if info.Mode().IsRegular() && (info.Mode()&0o111) != 0 {
			return t.SetResult(tcl.RetOk, "1")
		}
	}

	return t.SetResult(tcl.RetOk, "0")
}

// Join parts of a name with system delimiter.
func fileJoin(t *tcl.Tcl, args []string) int { // name name?
	// filepath.Join
	if len(args) < 3 {
		return t.SetResult(tcl.RetError, "file "+args[1]+" name ?name")
	}
	dirPath := []string{}
	for _, n := range args[2:] {
		if n[0] == filepath.Separator {
			dirPath = []string{}
		} else {
			dirPath = append(dirPath, n)
		}
	}
	if len(dirPath) == 0 {
		return t.SetResult(tcl.RetOk, "")
	}
	return t.SetResult(tcl.RetOk, filepath.Join(dirPath...))
}

// Change working directory.
func fileCwd(t *tcl.Tcl, args []string) int {
	if len(args) != 3 {
		return t.SetResult(tcl.RetError, "file cd dir")
	}
	err := os.Chdir(args[2])
	if err != nil {
		return t.SetResult(tcl.RetOk, err.Error())
	}
	return t.SetResult(tcl.RetOk, "")
}

// Print working directory.
func filePwd(t *tcl.Tcl, args []string) int {
	if len(args) != 2 {
		return t.SetResult(tcl.RetError, "file pwd")
	}
	dir, err := os.Getwd()
	if err != nil {
		return t.SetResult(tcl.RetOk, err.Error())
	}
	return t.SetResult(tcl.RetOk, dir)
}

// Make a directory.
func fileMkdir(t *tcl.Tcl, args []string) int { //  dir?
	if len(args) < 2 {
		return t.SetResult(tcl.RetError, "file mkdir dir ?dir")
	}

	i := 2

	for len(args) > i {
		fmt.Println("mkdir ", len(args), i, args[i])
		err := os.Mkdir(args[i], 0o750)
		if err != nil {
			return t.SetResult(tcl.RetError, err.Error())
		}
		i++
	}
	return t.SetResult(tcl.RetOk, "")
}

// Read a directory.
func fileDir(t *tcl.Tcl, args []string) int {
	all := false
	i := 2

	if len(args) < i {
		return t.SetResult(tcl.RetError, "file dir -all ?dir")
	}

	if len(args) > i && args[i] == "-all" {
		all = true
		i++
	}

	dir := "."
	if len(args) > i {
		dir = args[i]
	}
	dirList, err := os.ReadDir(dir)
	if err != nil {
		return t.SetResult(tcl.RetError, err.Error())
	}
	res := []string{}

	for _, file := range dirList {
		if all || file.Name()[0] != '.' {
			res = append(res, file.Name())
		}
	}

	return t.SetResult(tcl.RetOk, strings.Join(res, " "))
}

// Rename a file.
func fileRename(t *tcl.Tcl, args []string) int { // -force -- source target
	if len(args) < 2 {
		return t.SetResult(tcl.RetError, "file rename ?-force file ?file ?target")
	}

	force := false

	i := 2
	if args[i] == "-force" {
		force = true
		i++
	}

	if len(args) < (i + 2) {
		return t.SetResult(tcl.RetError, "file rename ?-force file ?file ?target")
	}

	// Check if last argument is a directory.
	target := args[len(args)-1]
	dir := false

	stat, err := os.Stat(target)
	if err == nil {
		if stat.IsDir() {
			dir = true
		}

		if stat.Mode().IsRegular() && !force {
			return t.SetResult(tcl.RetError, "file exists and is not direcory")
		}
	}

	for len(args) > (i + 1) {
		fmt.Println("rename ", target, len(args), i, args[i])
		if dir {
			err = os.Rename(args[i], target+string(os.PathSeparator)+filepath.Base(args[i]))
		} else {
			err = os.Rename(args[i], target)
		}
		if err != nil {
			return t.SetResult(tcl.RetError, err.Error())
		}
		i++
	}
	return t.SetResult(tcl.RetOk, "")
}

// Return file system separator.
func fileSeparator(t *tcl.Tcl, args []string) int {
	if len(args) > 2 {
		return t.SetResult(tcl.RetError, "file "+args[1])
	}
	return t.SetResult(tcl.RetOk, string(filepath.Separator))
}

// Return access to file of file.
func fileAccess(t *tcl.Tcl, args []string) int { // name
	if len(args) > 3 {
		return t.SetResult(tcl.RetError, "file "+args[1]+" name")
	}

	switch args[1] {
	case "readable":
		f, err := os.OpenFile(args[2], os.O_RDONLY, 0o666)
		if err == nil {
			f.Close()
			return t.SetResult(tcl.RetOk, "1")
		}
	case "writable":
		f, err := os.OpenFile(args[2], os.O_WRONLY, 0o666)
		if err == nil {
			f.Close()
			return t.SetResult(tcl.RetOk, "1")
		}
	}
	return t.SetResult(tcl.RetOk, "0")
}
