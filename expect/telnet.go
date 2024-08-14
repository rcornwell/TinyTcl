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
	"fmt"
	"net"

	"github.com/rcornwell/tinyTCL/tcl"
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

	tnStateData int = 1 + iota // normal
	tnStateIAC                 // IAC seen
	tnStateWILL                // WILL seen
	tnStateDO                  // DO seen
	tnStateDONT                // DONT seen
	tnStateWONT                // WONT seen
	tnStateSKIP                // skip next cmd
	tnStateSB                  // Start of SB expect type
	tnStateSE                  // Waiting for SE
	tnStateSBIS                // Waiting for IS
	// tnStateWaitVar                // Wait for Var or Value.
	tnStateSBData // Data for SB until IS
	tnStateSTerm  // Grab terminal type
	// tnStateEnv                    // Grab environment type.

	// tnStateUser // Grab user name.

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

// var initString = []byte{
// 	tnIAC, tnWONT, tnOptionLINE,
// 	tnIAC, tnWILL, tnOptionEcho,
// 	tnIAC, tnWILL, tnOptionSGA,
// 	tnIAC, tnWILL, tnOptionBinary,
// 	tnIAC, tnDO, tnOptionTerm,
// }

// Convert option number to string.
func optName(opt byte) string {
	switch opt {
	case tnOptionBinary:
		return "bin"
	case tnOptionEcho:
		return "echo"
	case tnOptionSGA:
		return "sga"
	case tnOptionTerm:
		return "term"
	case tnOptionEOR:
		return "eor"
	case tnOptionNAWS:
		return "naws"
	case tnOptionLINE:
		return "line"
	case tnOptionENV:
		return "env"
	}
	return tcl.ConvertNumberToString(int(opt), 10)
}

type tnState struct {
	optionState [256]uint8 // Current state of telnet session
	sbtype      byte       // Type of SB being received
	state       int        // Current line State
	conn        net.Conn   // Client connection.
}

// Send a response to server, and log what we sent.
func (state *tnState) sendOption(setState, option byte) {
	data := []byte{tnIAC, setState, option}
	_, _ = state.conn.Write(data)
	switch setState {
	case tnWILL:
		state.optionState[option] |= tnFlagWill
		fmt.Println("Send Will " + optName(option))
	case tnWONT:
		state.optionState[option] |= tnFlagWont
		fmt.Println("Send Wont " + optName(option))
	case tnDO:
		state.optionState[option] |= tnFlagDo
		fmt.Println("Send Do " + optName(option))
	case tnDONT:
		state.optionState[option] |= tnFlagDont
		fmt.Println("Send Dont " + optName(option))
	}
}

// Handle DO response.
func (state *tnState) handleDO(input byte) {
	switch input {
	case tnOptionTerm:
		fmt.Println("Do Term")
		if (state.optionState[input] & tnFlagWill) != 0 {
			state.optionState[input] |= tnFlagDont

		}
		state.sendOption(tnWONT, input)
	case tnOptionSGA:
		fmt.Println("Do SGA")
		if (state.optionState[input] & tnFlagWill) != 0 {
			state.optionState[input] |= tnFlagDont
		}
	case tnOptionEcho:
		fmt.Println("Do Echo")
		if (state.optionState[input] & tnFlagWill) != 0 {
			state.optionState[input] |= tnFlagDont
		}
	case tnOptionEOR:
		fmt.Println("Do EOR")
		state.optionState[input] |= tnFlagDo
	case tnOptionBinary:
		fmt.Println("Do Binary")
		if (state.optionState[input] & tnFlagDo) == 0 {
			state.sendOption(tnDO, input)
		}
	default:
		fmt.Println("Do unknown")
		if (state.optionState[input] & tnFlagWont) == 0 {
			state.sendOption(tnWONT, input)
		}
	}
}

// Handle WILL response.
func (state *tnState) handleWILL(input byte) {
	//switch input {
	// case tnOptionTerm: // Collect option
	// 	fmt.Println("Will Term")
	// 	if (state.optionState[input] & tnFlagWill) == 0 {
	// 		state.optionState[input] |= tnFlagWill
	// 		state.sendOption(tnWONT, input)
	// 	}
	// case tnOptionENV:
	// 	if (state.optionState[input] & tnFlagWill) == 0 {
	// 		state.optionState[input] |= tnFlagWill
	// 		state.sendOption(tnWONT, input)
	// 	}
	// case tnOptionEOR:
	// 	fmt.Println("Will EOR")
	// 	if (state.optionState[input] & tnFlagWill) == 0 {
	// 		state.optionState[input] |= tnFlagWill
	// 	}
	// case tnOptionSGA:
	// 	fmt.Print("Will SGA")
	// 	if (state.optionState[input] & tnFlagWill) == 0 {
	// 		state.sendOption(tnDO, input)
	// 	}
	// case tnOptionEcho:
	// 	fmt.Println("Will Echo")
	// 	if (state.optionState[input] & tnFlagWill) == 0 {
	// 		state.optionState[input] |= tnFlagWill
	// 		state.sendOption(tnDONT, input)
	// 		state.sendOption(tnWONT, input)
	//		}
	// case tnOptionBinary:
	// 	fmt.Println("Will Bin")
	// 	if (state.optionState[input] & tnFlagWill) == 0 {
	// 		state.optionState[input] |= tnFlagWill
	// 	}
	//	default:
	if (state.optionState[input] & tnFlagDont) == 0 {
		state.sendOption(tnDONT, input)
	}
	// }
}

func (state *tnState) handleSE(term []byte) {
}

func openTelnet(conn net.Conn) *tnState {
	state := tnState{conn: conn, state: tnStateData}
	// state.sendOption(tnOptionSGA, tnDO)
	// state.sendOption(tnOptionLINE, tnWILL)
	// state.sendOption(31, tnWONT)
	// state.sendOption(35, tnWONT)
	//state.sendOption(tnOptionTerm, tnWONT)
	return &state
}

func (tn *tnState) sendTelnet(output []byte) error {
	buffer := []byte{}
	for i := range output {
		buffer = append(buffer, output[i])
		if output[i] == tnIAC {
			buffer = append(buffer, tnIAC)
		}
	}
	_, err := tn.conn.Write(buffer)
	return err
}

func (tn *tnState) receiveTelnet(input []byte, len int) []byte {
	out := []byte{}

	for i := range len {
		ch := input[i]
		switch tn.state {
		case tnStateData: // normal
			if ch == tnIAC {
				tn.state = tnStateIAC
				fmt.Println("data: IAC")
			} else {
				//			fmt.Printf("data: %02x %c\n", input, input)
				out = append(out, ch)
			}
		// Otherwise send character to device.
		case tnStateIAC: // IAC seen
			switch ch {
			case tnIAC:
				// Send character to device
				tn.state = tnStateData
				fmt.Println("IAC")
			case tnBRK:
				tn.state = tnStateData
				//		fmt.Println("BRK")
			case tnWILL:
				tn.state = tnStateWILL
				//		fmt.Println("WILL")
			case tnWONT:
				tn.state = tnStateWONT
				//		fmt.Println("WONT")
			case tnDO:
				tn.state = tnStateDO
				//		fmt.Println("DO")
			case tnDONT:
				tn.state = tnStateDONT
				//		fmt.Println("DONT")
			case tnSB:
				tn.state = tnStateSB
				//		fmt.Println("SB")
			default:
				//		fmt.Printf("IAC Char: %02x\n", input)
				tn.state = tnStateSKIP
			}

		case tnStateWILL: // WILL seen
			fmt.Printf("Will %s\n", optName(ch))
			tn.handleWILL(ch)
			tn.state = tnStateData

		case tnStateWONT: // WONT seen
			fmt.Printf("Wont %s\n", optName(ch))
			if (tn.optionState[ch] & tnFlagWont) == 0 {
				tn.sendOption(tnWONT, ch)
			}
			tn.state = tnStateData

		case tnStateDO: // DO seen
			fmt.Printf("Do %s\n", optName(ch))
			tn.handleDO(ch)
			tn.state = tnStateData

		case tnStateDONT:
			fmt.Printf("Dont %s\n", optName(ch))
			tn.state = tnStateData

		case tnStateSKIP: // skip next cmd
			fmt.Print("Skip")
			tn.state = tnStateData

		case tnStateSB: // Start of SB expect type
			fmt.Printf("SB: %s\n", optName(ch))
			tn.sbtype = ch
			tn.state = tnStateSBIS

		case tnStateSBIS: // Waiting for IS
			fmt.Printf("SB IS %s\n", optName(tn.sbtype))
			switch tn.sbtype {
			case tnOptionTerm:
				tn.state = tnStateSTerm
				//				case tnOptionENV:
				//					state.state = tnStateWaitVar
			default:
				tn.state = tnStateSE
			}

		case tnStateSTerm:
			if ch == tnIAC {
				tn.state = tnStateSE
				//		fmt.Println("term type: ", string(term))
			}
		// case tnStateWaitVar:
		// 	switch input {
		// 	case tnVar:
		// 		fmt.Println("VAR")
		// 		state.state = tnStateEnv
		// 	case tnValue:
		// 		fmt.Println("Value")
		// 		state.state = tnStateUser
		// 	case tnIAC:
		// 		fmt.Println("IAC")
		// 		state.state = tnStateSE
		// 	default:
		// 		fmt.Println("Input")
		// 		state.state = tnStateData
		// 	}
		// case tnStateEnv:
		// 	fmt.Printf("env: %02x %c\n", input, input)
		// 	switch input {
		// 	case tnIAC:
		// 		state.state = tnStateSE
		// 	case tnValue:
		// 		state.state = tnStateUser
		// 	default:
		// 	}
		// case tnStateUser:
		// 	fmt.Printf("user: %02x %c\n", input, input)
		// 	switch input {
		// 	case tnIAC:
		// 		state.state = tnStateSE
		// 		fmt.Println("user: ", string(state.luname))
		// 	case tnVar:
		// 		state.state = tnStateEnv
		// 		fmt.Println("var user: ", string(state.luname))
		// 	default:
		// 		state.luname = append(state.luname, input)
		// 	}
		case tnStateSE:
			if ch == tnSE {
				tn.state = tnStateData
				fmt.Println("SE")
			}
		}
	}
	return out
}
