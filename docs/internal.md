# Internal operation

## Basic TCL

In TCl everything is a string. Commands are string items separated by blanks.
To place blanks in strings, TCL offers three methods. Double quotes, braces 
and brackets.Variables are referenced by $ variable name. For double quotes,
variables are expanded. Backslashes will allow the following character to be
converted to a special character or allow for the insertion of double quotes
or dollar signs. This resulting string is considered one parameter. For square
brackets the string is taken as a command to execute. If this occurs in a
double quote string it will be evaluated at that time. The results will be put
into the string. For braces dollar signs are not and brackets are not expanded.

## Environment structure

This struct holds the information about the current environment. In cmds is a
map of the commands and procs defined and the function to execute them. The
variables for this level are kept in env. For each level there is a map of
variable strings, wether the variable is local. And the parent of the current
level, this is used when upvar is called. When global variables are referenced
in a user proc the variable object is attached to the vars map in the current
environment, this allows for modification of variables to effect the values in
that level.

## Execution of commands

When commands are executed they are given a pointer to the current environment
and an array of strings for the arguments. The first element is the command 
name itself. Commands should end with a call to SetResult with the return
status, RetOk or RetError are the most common. Anything other then RetOk will
exit the current Eval function with the returned value. The second argument to
SetResult is the string to be returned from the command.

To add new commands to the current tcl environment Register is called. It is
passed a name and a function to execute when the command is to be called.

When a user created a proc, a function is created to call the command and 
attach the args and body of the procedure and placed into the cmd map. When
the proc is executed a new variable environment is created and the arguments
are scanned with ParseArgs to create a list of named arguments. For each name
the corresponding entry in the command arguments a variable with that name
and value is created. If the last variable is named "args" the remainder of
the arguments are stored in a variable named "args". After which the newly
created environment replaces the current environment with parent set to the
previous environment. The body of the procedure is now evaluated as a
command and after finished the the previous environment is restored. If the
body exited with a "return" command it is converted to an Ok return.

## Eval

Eval is the heart of the system. It takes a string and calls the parser to
collect tokens and return the results of executing the command. Depending on
the token various routines will be called. For variables the value is
attempted to be read. If it is not defined a error is returned. For commands
eval is called to process the returned string. The result of the command is
appended to the command currently being scanned. For escape the any '\'
characters are converted to their respective values. If space save as
previous token.

When End of line is detected the if noEval option is set the results are
joined into a string. Otherwise doCommand is called to try and execute the
command. If the previous token was a space or end of line the resulting
string is considered a new argument is appended to the currently collected
command. Otherwise it is appended to the current string being collected.

When a command is being executed, doCommand is called which looks up the
command in the cmds map. If it is found the given function is called with
the current tcl environment, and the array of arguments collected.

## ParseArgs

The ParseArgs function can be used by commands to split a string into pieces
at blank separators. Bracketed and braced terms are considered strings and
not expanded. No commands or variables are expanded.

## Parser

newParser takes a string along with options. Then a call to getToken will
return the next string element. GetToken looks at the first character to
determine which routine to call. If there is a blank or tab character and
the parser is not currently building up a quoted string, the following
blank characters are skipped by parseSeparator. ParseSeparator will skip
blanks, tabs, carriage returns and new line characters. Otherwise parseString
is called to continue scanning the current string.  If the first nonblank
character is a '[' up to the matching ']' character is scanned by parseCommand
it will be returned as a command token. If the parser option noCommands is set
the text will be returned as a string. If the string starts with a $
parseVariable to see if a variable name can be found. If the option noVars
is set the text is scanned as a string. Comments always start with '#', if a
string is not currently being processes. If anything else if found it is
treated as a string. If there is an error in the collection of the next token, 
GetToken will return false. If it gets a valid token it returns true.

The parse maintains a struct that holds information about the current string
being scanned. The current string is held in 'str', and the character
currently been looked at in 'nextPos'. This index is updated as each character
is scanned. Start and end hold the beginning and ending points of the current
match token. Inquote is a flag indicating that a quoted string is currently
being scanned. The text of the current token can be returned with a call to
GetString().

The routine next() will return character pointed to be nextPos if nextPos is
not over the length of the scanned string. Pos is set to the position of the
character read. If the character is a backslash and the next character is a
newline, it is skipped and next is called again to grab the next character.
When the end of string is reached the returned character is zero.

ParseSeparator() skips any blank characters and returns true to identify it
as valid blank string. ParseEOL() skips spaces and ; characters.

ParseCommand starts after the '[' is detected. Whenever a '[', '{' is detected
the string is continue to be scanned looking for the trailing '}' or ']'
character. When an unmatched ']' is found the command is considered to be
complete. If a '\' character is found, the next character is ignored. 

ParseVar scans text for a variable character or a string starting with '{' the
variable ends when the next '}' character is found. Valid variable names are
any string of letters, numbers or '_' characters. If the character following the '\$'
is not a brace or a variable character, the token is considered a string
of '$'.

ParseBrace is similar to parseCommand, however it only looks for matching '{'
'}' pairs.

ParseString scans the text looking for special tokens. If the last character
was a space, end of line or string, the following text is considered to be a
new string. If a brace is detected and the subst option is not set, parseBrace
is called to collect the next '{' '}' string. If the character is a '"' the
following is taken as a quoted string. If the character is a '\' the next
character is skipped. For '$' and noVars option is not set, the string is
considered an escape. If '[' and noCommands options is not set, the following
is considered an escape. If not in a quote and a separator or end of line
is detected the string is considered an escape. When a '"' is detected in
a quote the string is considered to have ended. 

ParseComment scans until and end of line character is found. If a '\'
character is detected and the next character is a newline, the comment is
considered to continue on the next line. 

## Helper functions

There are some functions that can be used to help in the writing of extension
commands. UnEscape(string) will take a string an convert "\" sequences to there
value.

ConvertStringToNumber takes the string to scan, the default base, and the
position in the string to start at. It recognizes leading '+' and '-' as
positive or negative number. Numbers starting with '0' are considered octal,
and starting with '0x' as hexadecimal. It will return the numeric value, the
last position scanned, and true or false depending on wether a number was
found or not.

ConvertNumberToString will take a number and a base and convert it to a number
that can be passed to ConvertStringToNumber. If the base is 8, a '0' is
prepended to the result, for base of 16 a '0x' is prepended.

SetVarValue takes a variable name and a value and either creates variable or
changes it's value.

UnSetVar will remove a variable from the environment.

GetVarValue will return the value of the variable or error if the variable is
not defined.

SetResult sets the result string for the operation and returns the return code
given.

GetResult returns the result of the last operation.

ParseArgs takes a string and expands it to an array of individual strings.
Variables and commands are not expanded.

StringEscape scans the string looking for special characters and encloses
the string in '{}' if there are any.

Match takes a pattern a string, flag wether to ignore case and the maximum
depth to attempt to match '*' character. It preforms the equivalent of 
globing on the string. '*' matches any number of characters, '?' matches the
next character, and '[]' will match any characters between the brackets. 

