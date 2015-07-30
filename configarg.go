package aoe

import (
	"encoding/binary"
	"io"
)

// A ConfigCommand is a subcommand used with a ConfigArg, as described in
// AoEr11, Section 3.2.
type ConfigCommand uint8

const (
	// ConfigCommandRead is used to read a server's config string.
	ConfigCommandRead ConfigCommand = 0

	// ConfigCommandTest is used to request that a server respond if and only
	// if the argument string exactly matches the server's config string.
	ConfigCommandTest ConfigCommand = 1

	// ConfigCommandTestPrefix is used to request that a server respond if and
	// only if the argument string is a prefix of the server's config string.
	ConfigCommandTestPrefix ConfigCommand = 2

	// ConfigCommandSet is used to set a server's config string, if and only
	// if the server's current config string is empty.
	ConfigCommandSet ConfigCommand = 3

	// ConfigCommandForceSet is used to forcibly set a server's config string,
	// and force it to respond.
	ConfigCommandForceSet ConfigCommand = 4
)

var (
	// Compile-time interface check
	_ Arg = &ConfigArg{}
)

// A ConfigArg is an argument to Command 1, Query Config Information,
// (CommandQueryConfigInformation) as described in AoEr11, Section 3.2.
type ConfigArg struct {
	// BufferCount specifies the maximum number of outstanding messages a
	// server can queue for processing.  Messages in excess of this value
	// are dropped.
	BufferCount uint16

	// FirmwareVersion specifies the version number of a server's firmware.
	FirmwareVersion uint16

	// SectorCount, if non-zero, specifies the maximum number of sectors a
	// server can handle in a single ATA command request.
	//
	// A value of 0 is equivalent to 2, for backward compatibility.
	SectorCount uint8

	// Version specifies the AoE protocol version a server supports.
	Version uint8

	// Command specifies the ConfigCommand carried in this argument.  Command
	// must be a 4-bit integer (0xf) or less.
	Command ConfigCommand

	// StringLength specifies the length of the String field.
	StringLength uint16

	// String specifies a server configuration string.  It can be no larger
	// than 1024 bytes.
	String []byte
}

const (
	// configArgLen specifies the minimum required length for a ConfigArg.
	//
	// 2 bytes: buffer count
	// 2 bytes: firmware version
	// 1 byte : sector count
	// 1 byte : version + config command
	//   0001 0001
	//   ^^^^ ^^^^
	//   |       +- config command
	//   +--------- version
	// 2 bytes: config string length
	// N bytes: config string
	configArgLen = 2 + 2 + 1 + 1 + 2
)

// MarshalBinary allocates a byte slice containing the data from a ConfigArg.
//
// If any of the following conditions occur, ErrorBadArgumentParameter is
// returned:
//   - c.Command is larger than a 4-bit integer (0xf)
//   - c.StringLength does not indicate the actual length of c.String
//   - c.StringLength is greater than 1024
func (c *ConfigArg) MarshalBinary() ([]byte, error) {
	// Command must be a 4-bit integer
	if c.Command > 0xf {
		return nil, ErrorBadArgumentParameter
	}

	// StringLength must indicate actual length of String
	if int(c.StringLength) != len(c.String) {
		return nil, ErrorBadArgumentParameter
	}

	// StringLength must not be greater than 1024, per AoEr11, Section 3.2.
	if c.StringLength > 1024 {
		return nil, ErrorBadArgumentParameter
	}

	// Allocate correct number of bytes for argument and config string
	b := make([]byte, configArgLen+int(c.StringLength))

	// Store basic information
	binary.BigEndian.PutUint16(b[0:2], c.BufferCount)
	binary.BigEndian.PutUint16(b[2:4], c.FirmwareVersion)
	b[4] = c.SectorCount

	// Set version in 4 most significant bits of byte 5; command in least
	// significant 4 bits
	var vc uint8
	vc |= c.Version << 4
	vc |= uint8(c.Command)
	b[5] = vc

	// Store config string length and string itself
	binary.BigEndian.PutUint16(b[6:8], c.StringLength)
	copy(b[8:], c.String)

	return b, nil
}

// UnmarshalBinary unmarshals a byte slice into a ConfigArg.
//
// If the byte slice does not contain enough data to form a valid ConfigArg,
// or config string length is greater than the number of remaining bytes in b,
// io.ErrUnexpectedEOF is returned.
//
// If config string length is greater than 1024, ErrorBadArgumentParameter is
// returned.
func (c *ConfigArg) UnmarshalBinary(b []byte) error {
	// Must contain minimum length for argument
	if len(b) < configArgLen {
		return io.ErrUnexpectedEOF
	}

	// Retrieve basic data
	c.BufferCount = binary.BigEndian.Uint16(b[0:2])
	c.FirmwareVersion = binary.BigEndian.Uint16(b[2:4])
	c.SectorCount = b[4]

	// Version is most significant 4 bits
	c.Version = uint8(b[5] >> 4)

	// Command is least significant 4 bits of byte 5
	c.Command = ConfigCommand(b[5] & 0x0f)

	// StringLength cannot be larger than the number of bytes remaining
	// in the buffer
	c.StringLength = binary.BigEndian.Uint16(b[6:8])
	if len(b[8:]) < int(c.StringLength) {
		return io.ErrUnexpectedEOF
	}
	// StringLength must not be greater than 1024, per AoEr11, Section 3.2.
	if c.StringLength > 1024 {
		return ErrorBadArgumentParameter
	}

	// Copy config string for use
	d := make([]byte, c.StringLength)
	copy(d, b[8:])
	c.String = d

	return nil
}
