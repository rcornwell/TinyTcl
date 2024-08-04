# Tiny TCL

Tiny Tcl is a Go version of TCL based on [picilo](https://github.com/howerj/pickle/), however this version has been
expanded to include many standard TCl operators. This is a 
pure interpreter, and supports only integer math. It is designed so that it can be embedded into an application. It can also easily be expanded to include new features. There is a sample of how an interpreter could be setup in main.go.

To set up an interpreter, a TCL environment must be first created.

    import (  
	    tcl "github.com/rcornwell/tinyTCL/tcl"  
    )  

	tcl := tcl.NewTCL()
	switch tcl.EvalString(command) {
    case  "quit":
		// Interpreter is done.
	case "error":
        // Error, message in the results.
    	fmt.Println("Error: " + tcl.GetResult())
	default:
        // Ok result, process as you will.
		fmt.Println(tcl.GetResult())
	}

Each call to EvalString will process one string.

### Basic Commands

More information about the syntax of these commands can be found on various Tcl help pages. Most options are supported, some which don't make sense have been left off. TinyTcl does not support arrays at this time, so commands referencing arrays have not been implemented.

append concat catch decr eq error eval exit expr incr join
ne puts set subst unset

### Control Flow

break continue for if switch while 

### Proceedures

proc rename return global upvar variable

### List commands

foreach lappend llength lindex linsert list lrange lreplace lsearch lset lsort split

### Extra commands
string info

## File Extension

To add commands for working with files to a TCL instance do:

	import (file "github.com/rcornwell/tinyTCL/tclfile")

	// Add in file commands.
	file.FileInit(tinyTcl)

This will add in:

close eof file flush gets open puts read seek tell

This extension also replaces the puts command to take a channel to write the message to.

## Adding new commands

To add new commands you will need to define a function of the form:  

       func command(*Tcl, []string) int

The first argument is the Tcl interpreter structure. The second argument is the command and any arguments passed to the function. 
Interpreter:

	   tcl.Register("name", function)

Procedure is true for user defined procedures, commands should set this to false.

The Tcl struct created by NewTcl() has one exported element. 

		Data   map[string]any     // Place for extensions to store data.

Extensions that need to hold data to be passed to various commands can create a struct and place a pointer to it there like:

	data := tclExtData{}
	tcl.Data["exten"] = &data

In a command proceedure this can be accessed by:

	data, ok := t.Data["exten"].(*tclExtData)
	if !ok {
		panic("invalid data type extension")
	}

If the data is not in the map, it means that you are being called from wrong instance of the interpreter and there is nothing more you can do. This should not happen, hence the panic.

## Helper functions

Convert string with backslash characters to one without backslash characters.

	// Process escape character.
	func UnEscape(str string) (string, int)

Converts a string with numbers in it, into numbers. The arguments are the string to evaluate, default base, and position in the string to start looking for a number. Returns are the converted number, then position to continue for more numbers, and true/false depending on whether a number was converted. Formats are +/-digits. 0 indicates octal, 0x indicates hex.

	func ConvertStringToNumber(str string, base int, pos int) (int, int, bool)

To convert a number into a string for returning it. This function takes number and base and returns the number in a format that ConvertStringToNumber can convert back.

	func ConvertNumberToString(num int, base int) string

SetVarValue, UnSetVar, GetVarValue can be used to set variables and get their values. GetVarValue returns tcl.RetOk if value was found or tcl.RetError if it does not exist.

	func (tcl *Tcl) SetVarValue(name string, value string)

	func (tcl *Tcl) UnSetVar(name string) 

	func (tcl *Tcl) GetVarValue(name string) (int, string) 

StringEscape converts a string to add in {} as needed.

	func StringEscape(str string) string 

Match does glob matches for a string. Return -1 if depth exceeded, 0 if no match, 1 if match.

	func Match(pat string, target string, nocase bool, depth int) int 

NewTCL creates a new TCL interpreter environment and initializes the default commands.

	func NewTCL() *Tcl 

GetResults, and SetResults can be used to get the results of a command execution or set the results. For SetResult err should be tcl.RetOk or tcl.RetError

	func (tcl *Tcl) SetResult(err int, str string) int 

	func (tcl *Tcl) GetResult() string

ParseArgs can be used to expand a string list into an array of values. 

	func (tcl *Tcl) ParseArgs(str string) []string


