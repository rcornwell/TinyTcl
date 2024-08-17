# Users Guide

## Overview

TinyTCL is similar to TCL, however it lacks many feature of standard TCL.
TinyTCL is meant to be used embedded in application. The main.go file is
only a sample of how to read in a file and run it, or provide interactive
test environment. The interpret only supports integer math, it does
not implement arrays and all of the options for various commands, regular
expressions. TinyTCL does not compile any code but is straight interpreter.
It is not meant as high performance implementation, but as simple extendable
interpreter.

For commands that take a conditional value, a 1, yes, true will be considered
a true value, a empty string, 0, no, false will be considered a false value.

## Basic supported commands

In TCL everything is a string. Variables begin with $ and can also be bracket
by {}. All strings surrounded by double quotes will have all variables expanded
before being considered a string. Strings surrounded by braces [] will be executed
either in "" or outside. Strings surrounded by braces {} will not be evaluated
until explicitly evaluated. Arguments are delimited by whitespace. Values that 
start with ? are optional parameters.

#### append varName ?args

Append takes a variable as it's first argument, it then concatenates the remaining
arguments to the end of the variable. It will return the new string.

#### break

Break will exit a for, while, foreach loop.


#### catch arg ?varName

Catch evaluates the argument and if there is an error puts the error string in 
varName if there is one. It will return 1 if there was an error in the evaluation
of the argument, or 0 if it was successful.

#### concat ?args

Concat combines all arguments into one string. Each argument is trimmed of spaces
before it is appended to result. 

#### continue

Continue will cause the current for/while/foreach loop to skip the remaining steps
and go back to revalue the condition.

#### decr varName ?value

Subtracts one or value from variable. The variable is updated to the new value. 
The result is the new value of variable.

#### eq string1 string2

Compares the two arguments and returns 1 if they match and 0 if they don't.

#### error message

Error returns a command error with the value of the message.

#### eval arg ?arg ...?

Eval combines arguments together and then treats them as a command. It 
returns whatever the command returned.

#### exit ?value

Exits the interpreter with value, if no value exit 0 status.

#### expr opr value  or expr value1 opr value2

Expression preforms the arithmetic operation on either one or two arguments.
Current operations supported are +,-,*,/,and,or,xor,max,min,>,>=,<,<=,==,!=
for binary operators. Unary operators are +/-/neg/not/inv/abs/bool. Too 
compare two strings, they must be separated by blanks. This is only valid
for relation operators.

#### for init cond increment body

For evaluates the init command. It then does [expr cond] if is true
will evaluate the body, followed by increment. Once [expr cont] is false
for will end. Executing the "break" command will terminate the loop. 
Executing the "continue" command will cause increment to be evaluated.

#### foreach varlist1 list1 ?varlist list...? body

Foreach loops over the varlist's setting each value in varlist to the
next value in list. For each iteration body is called.

#### global varlist?

Global makes each top level variable in varlist visible in the current 
function. If the value does not exist global returns an error.

#### if cond body ?elseif cond body?... ?else body?

If command takes a condition and body followed by as many elseif condition
body. Optionally it can be followed by an else. If the condition is true
the corresponding body will be executed. If no conditions are meet the
else body will be executed.

#### info type ?args

The info command can be used to determine information about the system. Type can
be one of the following:

- args procname      Returns the arguments to procname if procname exists.
- body procname      Returns the body of procname if procname exists.
- commands ?pattern  Returns a list of commands including proceedure.
                     If pattern given returns only the matching elements.
- exists varName     Returns 1 if varName exists, 0 if not.
- globals ?pattern   Returns a list of global variables. 
- level number      Returns arguments for proceedure running at number.
- local ?pattern    Returns local variable from current level.
- procs ?pattern    Returns list of user defined procs.
- vars ?pattern     Returns list of variables defined.

#### incr varName ?value

Adds one or value from variable. The variable is updated to the new value. 
The result is the new value of variable.

#### join list ?separator

Joins list elements with either blank or the separator if given.

#### lappend listVar ?args

Appends the args to list variable.

#### lindex list index

Returns the list item at index. Index can be "end" or "end-#" to start
from end rather than beginning of list.

#### linsert list index ?args

Inserts the following arguments into list at the given index.

#### list ?args

Returns a list built from the arguments. Arguments are quoted if
needed to be. 

#### llength list

Returns number of elements in the list.

#### lrange list start end

Returns elements in list from start to end as new list.

#### lreplace list start end ?args

Replaces elements in list between start and end. If args is empty, the
elements are deleted. If number of args is greater then start to end, 
the new elements are inserted.

#### lsearch ?options? list pattern

Searches for an element matching pattern in the given list. The following
flags are supported:
- -all return all elements matching pattern.
- -exact exact match element.
- -glob matches based on glob expressions(default).
- -inline return value of matches rather then the index.
- -integer compares elements as integers. 

- -nocase compare while ignoring case of pattern or elements.
- -not return elements not matching pattern.
- -regexp same as glob for the moment.
- -sort sorts the list in ascending order.
- -start position starts the search at position.

Normally lsearch returns the index(es) of the matched elements. If
-inline is specified the actual values are returned.

#### lset listVar index newValue

Replaces elements in listVar at the index given. If index is a list
of indices, all elements of the list are replaced with newValue. The
result of lset is a new list with values replaced, the listvar is
not modified.

#### lsort ?options? list

Sorts a list. Options can be given to change type of search.

- -ascii match elements as strings (default).
- -decreasing sort elements in decreasing order.
- -increasing sort elements in increasing order (default).
- -integer sort elements as integers rather then strings.
- -command proc call proc to compare elements.

#### ne string1 string2

Compares the two arguments and returns 0 if they match and 1 if they don't.

#### proc name args body

Creates a user proc (or command) that takes the list of arguments in args, and
executes the body when called. The proc is executed by name followed by arguments.
The args is a list of variable names that take on the value of each argument as
given. If the last name in the list is "args" then any elements remaining will be 
made into a list and set into the variable "args".

#### puts string

Puts prints the string on the standard output. Note this command is overridden if
the file extension is added.

#### rename name1 name2

Renames command or user procedure named name1 to name2.

#### return ?value

Returns from the user procedure with the argument value. If no value is given
the result is an empty string. 

#### set varName ?value

Sets varName to the value or empty string is value not given. Also creates a
variable if one of varName does not exist.

#### split string ?splitChars

Replaces every occurrence of splitChars in string. Returns a list containing the
new results. If splitChars is not given blank is assumed. If empty string is
given it will split every character into it's own element.

#### string option ?args

See next section.

#### subst ?options string

Preforms variable and command substitution on the command. Braces are treated 
as double quote. Options are:

- -nobackslashes Does not convert backslash characters to their value.
- -novariables Does not do variable expansion for $ elements.
- -nocommands Does not execute any [] commands.


#### switch ?options string pattern body?

Switch compares string to pattern when it finds a match it executes the
body. There can be as many pattern body pairs as needed. Also the pairs can
be passed as a list element enclosed in {}. Switch has the following options:

- -exact match string to pattern without using glob matching(default).
- -glob match string using glob expressions on string.
- -regexp same as glob for the moment.
- -- end options (used if string starts with -)


#### upvar ?level otherVar myVar ....

Copies variables from the given level (1 up if not given). Level can start with
a #, in which case it start searching from the top (0 being global level).
Each otherVar name is linked to myVar in the current procedure.

#### unset varName

Removes the variable varName from current level.

#### variable name ?value ....

Creates variables at the current level setting them to value. If the last name does
not specify a value, it is set to the empty string.

#### while cond body

Evaluates cond with expr, if condition is true, executes body. Continues until cond returns
false value.

## String command.

The string command accepts many options so each one can be considered a separate command.

#### string compare ?options string1 string2

Compares string1 to string2 returns -1 if string1 less then string2, 0 if string1 same as
string2, otherwise 1. Options are:

- -nocase   ignore case when matching.
- -length int Only match int number of characters.

#### string equal ?options string1 string2

Compares string1 to string2, same options as compare function. Returns 1 if strings match
else 0.

#### string first string1 string2 ?startIndex

Searches string2 for any occurrences of string1. If startIndex is given it will start
here rather than at beginning of string. If string1 does not appear in string2 return
-1.

#### string index string1 charIndex

Returns the character at index in string1. "end" can be used to start from end of
string1. If index less than 0 or greater then length return empty string.

#### string is class ?options string1

Check if string is a type of class. Return 1 if all characters in string1 are
members of the given class, else 0. If option -strict is given return 1 on empty.
If option -failindex varName is given, varName will be set to the index of the
first element that is not of the given class. Classes are:

- alnum Alphabetic or digit
- alpha Alphabetic
- ascii Any character less then 0x80.
- boolean any boolean form.
- control Any Unicode control character.
- digit Any digit.
- false Any false value.
- graph Any Unicode graphics character.
- lower Any lowercase letter.
- print Any Unicode printable character.
- punct Any Unicode punctuation.
- space Any Unicode blank.
- true Any true value.
- upper Any uppercase letter.

#### string last string1 string2 ?endIndex

Like first, but looks backward in string. 

#### string map ?-nocase mapping string1

Mapping is a list of string value pairs. Scan string1 looking for any
matches, and replace them with the new value. If there is no match, 
put the current character into result.

#### string match ?-nocase pattern string1

Uses glob expressions to match string1 with pattern. Returns true if
there is a match, false if not.

#### string range string1 firstIndex lastIndex

Returns a new string1 from firstIndex to lastIndex.

#### string repeat string1 count

Returns string1 count number of times.

#### string replace string1 first last ?newString

Replaces in string1 from first to last. If newString is given
that is inserted. If no string or empty string, the characters are
deleted.

#### string totitle string1 ?first ?last
#### string tolower string1 ?first ?last
#### string toupper string1 ?first ?last

Returns string converted to case from first to last. If last is not
given, convert to end of string. If first is given start converting
there. Last can't be given without giving first.

#### string trim string1 ?chars
#### string trimleft string1 ?chars
#### string trimright string1 ?chars

Trim characters off the beginning or end of string1. If chars is not
give use blanks.

## File extension

The file extension adds in commands to operate on files. It is also an
example of how to extend TinyTCL. TCL uses channels to handle open
files The extension adds the following commands:

#### close channel

Closes an open channel.

#### eof channel

Returns true if End of file has been detected on channel.

#### file command ?args

The file command options are discussed below.

#### flush channel

Forces any pending output to be written on channel.

#### gets channel ?varName

Reads in next line from channel. If varName is given, sets the result
into variable and returns the number of characters read. If varName is
not given, returns to line read in.

#### open name ?access ?perms

Opens a file, if no access is given the file is opened for reading. Access can be used
to specify how to open the file. perms is optional and is used on creating a file to
set access permissions. Default is (0o666). Access can be, r,r+,w,w+,a,a+. If + option
is given then the file is opened read/write. Returns the name of the channel openned.

#### read ?-nonewline channel numChars

Reads in numChars from channel, it strips the trailing newline character if -nonewline
is specified. Returns data read.

#### puts ?-nonewline ?channel string

Overwritten form basic system. If -nonewline option is given don't put newline character
at end of string. If channel is not given write string to stdout.

#### seek channel offset ?origin

Seeks to location offset into file given by channel. Origin can be: start, current, end
to specify where offset applies. Returns new position.

#### source name ?args

Reads in file named and runs any commands found. Args is set into the args variable.

#### tell channel

Tells position in file. Equivalent to "seek channel 0 current".

## File command.

The file command is used to return information about files or opened files. 

#### file atime name

Currently not implemented.

#### file channels ?pattern

Returns list of open files. If pattern is given only those matching pattern are returned.

#### file copy ?-force source... target

Copies either one file to another, or copies multiple files to directory. If the file exists
copy will return an error unless -force option is given.

#### file cwd dir

Change current working directory of current interpreter.

#### file delete -force file ?file

Deletes named files. Force currently is ignored.

#### file dir ?name

Returns list of files in directory. If name not given returns current directory.

#### file dirname name

Returns the directory part of the given name.

#### file executable name

Returns true if name is regular file and has one of the execute bits set.

#### file extension name

Returns the part of the filename after the last dot.

#### file isdir name

Returns true if name is a directory.

#### file isfile name

Returns true if name is a regular file.

#### file join name?

Joins all names with path separator.

#### file mkdir name?

Create directory for all named arguments.

#### file readable name

Tries to open file name as readable. If it successes return true.

#### file rename ?-force source... target

Renames source to target or moves the listed files to target if it is
a directory. If the file exists it will not be modified, unless
-force option is given.

#### file rootname name 

Returns the same as dirname.

#### file pwd

Returns the current working directory.

#### file separator

Returns system specific file path separater.

#### file size name

Returns size of file.

#### file split name


#### file tail name

Returns last part of path.

#### file type name

Returns the type of file name is.

#### file writable name

Tries to open the file for write, return true if success, false if not.


