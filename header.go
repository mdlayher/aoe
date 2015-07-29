package aoe

import (
	"encoding/binary"
	"io"
)

// A Header is an ATA over Ethernet header, as described in AoEr11, Section 2.
//
// In this package, a Header does not include the Ethernet header which
// encapsulates it during transport over a network.  When serving ATA over
// Ethernet requests, Ethernet headers are transparently added and removed as
// needed by this package.
type Header struct {
	// Version specifies the AoE version for this Header.  Version must
	// match the Version constant in this package.
	Version uint8

	// FlagResponse indicates if a message is a response to a request.
	FlagResponse bool

	// FlagError indicates if a command generated an AoE protocol error.
	FlagError bool

	// Error contains an Error value which can be used to report problems
	// to a client.
	Error Error

	// Major and Minor specify the major and minor address of an AoE server.
	//
	// The special major value "0xffff" and minor value "0xff" are used to
	// indicate a broadcast message to all AoE servers.
	Major uint16
	Minor uint8

	// Command specifies a Command value for this message.
	Command Command

	// Tag specifies a unique tag which a client can use to correlate responses
	// with their appropriate commands.
	Tag [4]byte

	// Arg specifies an argument field, which contains different types of
	// arguments depending on the value specified in Command.
	Arg Arg
}

const (
	// headerLen is the minimum required length for a valid Header.
	//
	// 1 byte : version + flags
	//   0001 1100
	//   ^^^^ ||
	//   |    |+-- error flag
	//   |    +--- response flag
	//   +-------- version
	// 1 byte : error
	// 2 bytes: major
	// 1 byte : minor
	// 1 byte : command
	// 4 bytes: tag
	// N bytes: arg
	headerLen = 1 + 1 + 2 + 1 + 1 + 4
)

// MarshalBinary allocates a byte slice containing the data from a Header.
//
// If h.Version is not Version (1), ErrorUnsupportedVersion is returned.
//
// If h.Arg is nil, ErrorBadArgumentParameter is returned.
func (h *Header) MarshalBinary() ([]byte, error) {
	// Version must be 1
	if h.Version != Version {
		return nil, ErrorUnsupportedVersion
	}

	// Arg must not be nil
	if h.Arg == nil {
		return nil, ErrorBadArgumentParameter
	}
	ab, err := h.Arg.MarshalBinary()
	if err != nil {
		return nil, err
	}

	// Allocate correct number of bytes for header and argument
	b := make([]byte, headerLen+len(ab))

	// Place Version in top 4 bits of first byte
	var vf uint8
	vf |= h.Version << 4

	// If needed, place response and error flags in bits 5 and 6 of first byte
	if h.FlagResponse {
		vf |= (1 << 3)
	}
	if h.FlagError {
		vf |= (1 << 2)
	}
	b[0] = vf

	// Store other fields directly in network byte order
	b[1] = uint8(h.Error)
	binary.BigEndian.PutUint16(b[2:4], h.Major)
	b[4] = h.Minor
	b[5] = uint8(h.Command)
	copy(b[6:10], h.Tag[:])

	// Copy argument data into end of header
	copy(b[10:], ab)

	return b, nil
}

// UnmarshalBinary unmarshals a byte slice into a Header.
//
// If the byte slice does not contain enough data to form a valid Header,
// or an argument is malformed, io.ErrUnexpectedEOF is returned.
//
// If the AoE version detected is not equal to the Version constant (1),
// ErrorUnsupportedVersion is returned.
//
// If an unknown Command type is present, ErrorUnrecognizedCommandCode is
// returned.
func (h *Header) UnmarshalBinary(b []byte) error {
	// Must contain minimum length for header
	if len(b) < headerLen {
		return io.ErrUnexpectedEOF
	}

	// Version must indicate Version constant (1, at time of writing)
	h.Version = uint8(b[0] >> 4)
	if h.Version != Version {
		return ErrorUnsupportedVersion
	}

	// Flags occupy bits 5 and 6 of first byte
	h.FlagResponse = (b[0] & 0x08) != 0
	h.FlagError = (b[0] & 0x04) != 0

	// Retrieve other fields stored in network byte order
	h.Error = Error(b[1])
	h.Major = binary.BigEndian.Uint16(b[2:4])
	h.Minor = b[4]
	h.Command = Command(b[5])

	tag := [4]byte{}
	copy(tag[:], b[6:10])
	h.Tag = tag

	// Determine Arg type using Command
	var a Arg
	switch h.Command {
	case CommandIssueATACommand:
		a = new(ATAArg)
	case CommandQueryConfigInformation:
		a = new(ConfigArg)
	case CommandMACMaskList:
		a = new(MACMaskArg)
	case CommandReserveRelease:
		a = new(ReserveReleaseArg)
	default:
		// Unknown Command type
		return ErrorUnrecognizedCommandCode
	}

	// Unmarshal Arg as proper type; this may also return io.ErrUnexpectedEOF
	// or other errors
	if err := a.UnmarshalBinary(b[10:]); err != nil {
		return err
	}
	h.Arg = a

	return nil
}
