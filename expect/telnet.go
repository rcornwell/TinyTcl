/*
 * TCL Telnet connection.
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
	"net"
)

// Telnet protocol constants - negatives are for init'ing signed char data

const (
	tnIAC     byte = 255 // protocol delim
	tnDONT    byte = 254 // dont
	tnDO      byte = 253 // do
	tnWONT    byte = 252 // wont
	tnWILL    byte = 251 // will
	tnSB      byte = 250 // Sub negotiations begin
	tnGA      byte = 249 // Go ahead
	tnIP      byte = 244 // Interrupt process
	tnBRK     byte = 243 // break
	tnSE      byte = 240 // Sub negotiations end
	tnIS      byte = 0
	tnSend    byte = 1
	tnInfo    byte = 2
	tnVar     byte = 0
	tnValue   byte = 1
	tnEsc     byte = 2
	tnUserVar byte = 3

	// Telnet line states.

	tnStateData   int = 1 + iota // normal
	tnStateIAC                   // IAC seen
	tnStateWILL                  // WILL seen
	tnStateDO                    // DO seen
	tnStateDONT                  // DONT seen
	tnStateWONT                  // WONT seen
	tnStateSKIP                  // skip next cmd
	tnStateSB                    // Start of SB expect type
	tnStateSE                    // Waiting for SE
	tnStateSBIS                  // Waiting for IS
	tnStateSBData                // Data for SB until IS
	tnStateSTerm                 // Grab terminal type

	// Telnet options.
	tnOptionBinary byte = 0  // Binary data transfer
	tnOptionEcho   byte = 1  // Echo
	tnOptionSGA    byte = 3  // Send Go Ahead
	tnOptionTerm   byte = 24 // Request Terminal Type
	tnOptionEOR    byte = 25 // Handle end of record
	tnOptionNAWS   byte = 31 // Negotiate about terminal size
	tnOptionLINE   byte = 34 // line mode
	tnOptionENV    byte = 39 // Environment

	// Telnet flags.
	tnFlagDo   uint8 = 0x01 // Do received
	tnFlagDont uint8 = 0x02 // Don't received
	tnFlagWill uint8 = 0x04 // Will received
	tnFlagWont uint8 = 0x08 // Wont received

)

type tnState struct {
	optionState [256]uint8 // Current state of telnet session
	sbtype      byte       // Type of SB being received
	state       int        // Current line State
	conn        net.Conn   // Client connection.
}

// Create new telnet object.
func openTelnet(conn net.Conn) *tnState {
	state := tnState{conn: conn, state: tnStateData}
	return &state
}

// Send output to telnet connection.
func (state *tnState) sendTelnet(output []byte) error {
	buffer := []byte{}
	for i := range output {
		buffer = append(buffer, output[i])
		if output[i] == tnIAC {
			buffer = append(buffer, tnIAC)
		}
	}
	_, err := state.conn.Write(buffer)
	return err
}

// Process input from telnet server.
func (state *tnState) receiveTelnet(input []byte, length int) []byte {
	out := []byte{}

	for i := range length {
		ch := input[i]
		switch state.state {
		case tnStateData: // normal
			if ch == tnIAC {
				state.state = tnStateIAC
			} else {
				out = append(out, ch)
			}
		// Otherwise send character to device.
		case tnStateIAC: // IAC seen
			switch ch {
			case tnIAC:
				// Send character to device
				state.state = tnStateData
			case tnBRK:
				state.state = tnStateData
			case tnWILL:
				state.state = tnStateWILL
			case tnWONT:
				state.state = tnStateWONT
			case tnDO:
				state.state = tnStateDO
			case tnDONT:
				state.state = tnStateDONT
			case tnSB:
				state.state = tnStateSB
			default:
				state.state = tnStateSKIP
			}

		case tnStateWILL: // WILL seen
			state.handleWILL(ch)
			state.state = tnStateData

		case tnStateWONT: // WONT seen
			if (state.optionState[ch] & tnFlagWont) == 0 {
				state.sendOption(tnWONT, ch)
			}
			state.state = tnStateData

		case tnStateDO: // DO seen
			state.handleDO(ch)
			state.state = tnStateData

		case tnStateDONT:
			state.state = tnStateData

		case tnStateSKIP: // skip next cmd
			state.state = tnStateData

		case tnStateSB: // Start of SB expect type
			state.sbtype = ch
			state.state = tnStateSBIS

		case tnStateSBIS: // Waiting for IS
			state.state = tnStateSE

		case tnStateSTerm:
			if ch == tnIAC {
				state.state = tnStateSE
			}

		case tnStateSE:
			if ch == tnSE {
				state.state = tnStateData
			}
		}
	}
	return out
}

// Send a response to server, and log what we sent.
func (state *tnState) sendOption(setState, option byte) {
	data := []byte{tnIAC, setState, option}
	_, _ = state.conn.Write(data)
	switch setState {
	case tnWILL:
		state.optionState[option] |= tnFlagWill
	case tnWONT:
		state.optionState[option] |= tnFlagWont
	case tnDO:
		state.optionState[option] |= tnFlagDo
	case tnDONT:
		state.optionState[option] |= tnFlagDont
	}
}

// Handle DO response.
func (state *tnState) handleDO(input byte) {
	switch input {
	case tnOptionTerm:
		if (state.optionState[input] & tnFlagWill) != 0 {
			state.optionState[input] |= tnFlagDont
		}
		state.sendOption(tnWONT, input)
	case tnOptionSGA:
		if (state.optionState[input] & tnFlagWill) != 0 {
			state.optionState[input] |= tnFlagDont
		}
	case tnOptionEcho:
		if (state.optionState[input] & tnFlagWill) != 0 {
			state.optionState[input] |= tnFlagDont
		}
	case tnOptionEOR:
		state.optionState[input] |= tnFlagDo
	case tnOptionBinary:
		if (state.optionState[input] & tnFlagDo) == 0 {
			state.sendOption(tnDO, input)
		}
	default:
		if (state.optionState[input] & tnFlagWont) == 0 {
			state.sendOption(tnWONT, input)
		}
	}
}

// Handle WILL response.
func (state *tnState) handleWILL(input byte) {
	if (state.optionState[input] & tnFlagDont) == 0 {
		state.sendOption(tnDONT, input)
	}
}
