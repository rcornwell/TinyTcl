/*
 * TCL Parser
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
	"unicode"
)

const (
	tokCmd = 1 + iota
	tokEscape
	tokVar
	tokString
	tokEOL
	tokSpace
	tokOk
	tokError
	tokEOF
)

type parserOptions struct {
	noCommands bool // Don't execute commands.
	noEscapes  bool // Don't unescape symbols.
	noVars     bool // No variable expansion.
	noEval     bool // Treat {} as normal characters.
	subst      bool // Doing substitution.
}

type parser struct {
	str     string        // Text being parsed.
	pos     int           // Position in string.
	nextPos int           // Next character to read.
	char    byte          // Current character being processed.
	start   int           // Start of token.
	end     int           // End of token.
	inQuote bool          // In quote.
	token   int           // Current token.
	options parserOptions // Options for this parser.
}

// Create new parser.
func newParser(str string, options parserOptions) *parser {
	return &parser{str: str, char: str[0], nextPos: 1, token: tokEOL, options: options}
}

// Collect next token.
func (p *parser) getToken() bool {
	for p.char != 0 {
		switch p.char {
		case ' ', '\t':
			if p.inQuote {
				return p.parseString()
			}
			return p.parseSeperator()

		case '\n', '\r', ';':
			if p.inQuote {
				return p.parseString()
			}
			return p.parseEol()

		case '[':
			r := p.parseCommand()
			if r && p.token == tokCmd && p.options.noCommands {
				// Grab []'s and return string.
				p.start--
				p.end++
				p.token = tokString
				return true
			}
			return r

		case '$':
			if p.options.noVars {
				return p.parseString()
			}
			return p.parseVar()

		case '#':
			if p.token == tokEOL {
				if !p.parseComment() {
					return false
				}
				continue
			}
			return p.parseString()

		default:
			return p.parseString()
		}
	}

	if p.token != tokEOL && p.token != tokEOF {
		p.token = tokEOL
	} else {
		p.token = tokEOF
	}
	return true
}

// Get last matched token.
func (p *parser) GetString() string {
	if p.start == p.end {
		return ""
	}
	return p.str[p.start:p.end]
}

const hex = "0123456789abcdef"

// Advance to next character.
func (p *parser) next() {
	if p.nextPos < len(p.str) {
		p.pos = p.nextPos
		p.char = p.str[p.pos]
		p.nextPos++
		// Check if we got a \\ \n pair, if so skip them.
		if p.char == '\\' {
			if p.nextPos < len(p.str) && p.str[p.nextPos] == '\n' {
				p.nextPos++
				p.next()
				return
			}
		}
	} else {
		p.char = 0
		p.pos = len(p.str)
	}
}

// Check if space character.
func (p *parser) parseIsSpace() bool {
	return p.char == ' ' || p.char == '\t' || p.char == '\n' || p.char == '\r'
}

// Skip over spaces. Return true if end of string.
func (p *parser) parseSeperator() bool {
	p.start = p.pos
	for p.parseIsSpace() {
		p.next()
	}
	p.end = p.pos
	p.token = tokSpace
	return true
}

// Scan end of line.
func (p *parser) parseEol() bool {
	p.start = p.pos
	for p.parseIsSpace() || p.char == ';' {
		p.next()
	}
	p.end = p.pos
	p.token = tokEOL
	return true
}

// Collect command. Start just after [.
func (p *parser) parseCommand() bool {
	p.next()
	p.start = p.pos
	blevel := 0
	level := 1
outer:
	for p.char != 0 {
		switch p.char {
		case '[':
			if blevel == 0 {
				level++
			}

		case ']':
			if blevel == 0 {
				level--
				if level == 0 {
					break outer
				}
			}

		case '\\': // Skip any escape character.
			p.next()

		case '{':
			blevel++

		case '}':
			if blevel != 0 {
				blevel--
			}
		case 0:
			break outer
		}
		p.next()
	}
	if p.char != ']' {
		return false
	}
	// Set length and skip the ].
	p.end = p.pos
	p.token = tokCmd
	p.next()
	return true
}

// Variable character.
func (p *parser) isVarChar() bool {
	return unicode.IsLetter(rune(p.char)) || unicode.IsDigit(rune(p.char)) || p.char == '_'
}

// Collect variable. Start on $.
func (p *parser) parseVar() bool {
	brace := false
	p.next()

	if p.char == '{' { // If ${ name }
		p.next()
		brace = true
	}

	p.start = p.pos
	for p.isVarChar() {
		p.next()
	}
	p.end = p.pos
	if brace {
		if p.char != '}' {
			return false
		}
	}
	if !brace && p.start == p.pos { // Just single character string "$"
		p.start = p.pos
		p.token = tokString
	} else {
		p.token = tokVar
	}
	return true
}

// Collect { text }. Start on {.
func (p *parser) parseBrace() bool {
	p.next()
	p.start = p.pos
	level := 1
	for {
		switch p.char {
		case '\\':
			if p.nextPos < len(p.str) { // Must have at least 2 characters.
				p.next()
			} else {
				return false
			}

		case '}':
			// Close }, decrease level until zero.
			level--
			if level == 0 {
				p.end = p.pos
				p.token = tokString
				p.next()
				return true
			}
		case '{':
			// Increase nesting level.
			level++
		case 0:
			return false
		}
		p.next()
	}
}

// Collect the following string.
func (p *parser) parseString() bool {
	// If previous was Space, End of Line or String, start new word.
	newWord := p.token == tokSpace || p.token == tokEOL || p.token == tokString
	// If new word and brace, parse the { string }.
	if newWord && p.char == '{' && !p.options.subst {
		return p.parseBrace()
	}

	// Start of word, and ", signal we are in "string", get next char.
	if newWord && p.char == '"' {
		p.inQuote = true
		p.next()
	}

	p.start = p.pos
	// Scan until end of string.
	for p.char != 0 {
		switch p.char {
		case '\\':
			// If not escaping, make sure at least 2 characters remain.
			if !p.options.noEscapes {
				if p.nextPos < len(p.str) { // Must have at least 2 characters.
					p.next()
				} else {
					return false
				}
			}

		case '$':
			// If pressing variables, everything up to $.
			if !p.options.noVars {
				p.end = p.pos
				p.token = tokEscape
				return true
			}

		case '[':
			// If doing commands, make start of command
			if !p.options.noCommands {
				p.end = p.pos
				p.token = tokEscape
				return true
			}

		case ' ', '\t', ';', '\n', 0:
			// Blanks, if not in quoted string, return what we got.
			if !p.inQuote {
				p.end = p.pos
				p.token = tokEscape
				return true
			}

		case '"':
			// Got a quote, mark end of string.
			if p.inQuote {
				p.end = p.pos
				p.token = tokEscape
				p.inQuote = false
				p.next()
				return true
			}
		}
		p.next()
	}

	// If we hit end of line return error.
	if p.inQuote {
		return false
	}

	// Return string we got
	p.end = p.pos
	p.token = tokEscape
	return true
}

// Skip rest of line.
func (p *parser) parseComment() bool {
	for p.char != '\n' && p.char != 0 {
		if p.char == '\\' && p.str[p.pos+1] == '\n' { // skip \ eol
			p.next()
		}
		p.next()
	}
	return true
}
