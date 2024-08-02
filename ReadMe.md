# Tiny TCL

Tiny Tcl is a Go version of TCL based on [picilo](https://github.com/howerj/pickle/), however this version has been
expanded to include many standard TCl operators. This is a 
pure interpreter, and supports only integer math. It is designed so that it can be embedded into an application. It can also easily be expanded to include new features. There is a sample of how an interpreter could be setup in main.go.

To set up an interpreter, a TCL enviorment must be first created.

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

append concat catch decr eq error eval exit expr incr join
ne puts set subst unset

### Control Flow

break continue for if switch while 

### Proceedures

proc rename return global upvar variable

### List commands

lappend llength lindex linsert list lrange lreplace lsearch lset lsort split

### Extra commands
string info

## Adding new commands

To add new commands you will need to define a function of the form:  

       func command(*tcl, []string, []string) int

The first argument is the Tcl interpreter structure. The second argument is the command and any arguments passed to the function. The last argument comes from the register function. It can be used to pass data or parameters to the command. To register a command with the Interpreter:

       tcl.Register("name", []string{arguments}, procedure, function)

Proceedure is true for user defined proceedures, commands should set this to false. For user defined proceedures arguments holds the parameters, and body of function.

## Helper functions

Convert string with backslash characters to one without backslash characters.

	// Process escape character.
	func UnEscape(str string) (string, int)

Converts a string with numbers in it, into numbers. The arguments are the string to evaluate, default base, and position in the string to start looking for a number. Returns are the converted number, then position to continue for more numbers, and true/false depending on whether a number was conveted. Formats are +/-digits. 0 indicates octal, 0x indicates hex.

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

NewTCL creates a new TCL interpreter enviorment and initializes the default commands.

	func NewTCL() *Tcl 

GetResults, and SetResults can be used to get the results of a command execution or set the results. For SetResult err should be tcl.RetOk or tcl.RetError

	func (tcl *Tcl) SetResult(err int, str string) int 

	func (tcl *Tcl) GetResult() string

ParseArgs can be used to expand a string list into an array of values. 

	func (tcl *Tcl) ParseArgs(str string) []string


