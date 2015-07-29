package aoe

import (
	"io"
	"net"
)

const (
	// macMaskArgLen specifies the minimum required length for a MACMaskArg.
	//
	//   1 byte : reserved
	//   1 byte : MAC mask command
	//   1 byte : MAC mask error
	//   1 byte : directive count
	// 8*N bytes: directives
	macMaskArgLen = 1 + 1 + 1 + 1

	// directiveLen specifies the exact required length for a Directive.
	//
	// 1 bytes: reserved
	// 1 byte : directive command
	// 6 bytes: ethernet address
	directiveLen = 1 + 1 + 6
)

// A DirectiveCommand is a subcommand used with a Directive, as described in
// AoEr11, Section 3.3.
type DirectiveCommand uint8

const (
	// DirectiveCommandNone indicates that no command should be processed for
	// a Directive.
	DirectiveCommandNone DirectiveCommand = 0

	// DirectiveCommandAdd indicates that the MAC address in a Directive should
	// be added to a server's MAC mask list.
	DirectiveCommandAdd DirectiveCommand = 1

	// DirectiveCommandDelete indicates that the MAC address in a Directive
	// should be deleted from a server's MAC mask list.
	DirectiveCommandDelete DirectiveCommand = 2
)

// A MACMaskCommand is a subcommand used with a MACMaskArg, as described in
// AoEr11, Section 3.3.
type MACMaskCommand uint8

const (
	// MACMaskCommandRead reads a server's MAC mask list.
	MACMaskCommandRead MACMaskCommand = 0

	// MACMaskCommandEdit edits a server's MAC mask list.
	MACMaskCommandEdit MACMaskCommand = 1
)

// A MACMaskError is an error which occurs while processing a MACMaskArg's
// directive list, as described in AoEr11, Section 3.3.
type MACMaskError uint8

const (
	// MACMaskErrorUnspecified is returned when an unspecified error occurs
	// while processing a directive list.
	MACMaskErrorUnspecified MACMaskError = 1

	// MACMaskErrorBadCommand is returned when an unknown DirectiveCommand
	// is passed in a Directive.
	MACMaskErrorBadCommand MACMaskError = 2

	// MACMaskErrorListFull is returned when a server's MAC mask list is
	// completely full.
	MACMaskErrorListFull MACMaskError = 3
)

// A Directive is a directive which should be processed in a MACMaskArg, as
// described in AoEr11, Section 3.3.
type Directive struct {
	// Command specifies the specific command to be processed for this
	// Directive.
	Command DirectiveCommand

	// MAC specifies the hardware address of a device which should be added
	// or removed from a server's MAC mask list, depending on the value
	// of Command.
	MAC net.HardwareAddr
}

// MarshalBinary allocates a byte slice containing the data from a Directive.
//
// If d.MAC is not 6 bytes in length, ErrorBadArgumentParameter is returned.
func (d *Directive) MarshalBinary() ([]byte, error) {
	// Ethernet hardware addresses must be 6 bytes in length
	if len(d.MAC) != 6 {
		return nil, ErrorBadArgumentParameter
	}

	// Allocate fixed-length byte structure
	b := make([]byte, directiveLen)

	// 1 byte reserved

	// Add command copy hardware address into Directive
	b[1] = uint8(d.Command)
	copy(b[2:], d.MAC)

	return b, nil
}

// UnmarshalBinary unmarshals a raw byte slice into a Directive.
//
// If the byte slice does not contain exactly 8 bytes, io.ErrUnexpectedEOF
// is returned.
//
// If byte 0 (reserved byte) is not empty, ErrorBadArgumentParameter is
// returned.
func (d *Directive) UnmarshalBinary(b []byte) error {
	// Must be exactly 8 bytes
	if len(b) != directiveLen {
		return io.ErrUnexpectedEOF
	}

	// Byte 0 is reserved
	if b[0] != 0 {
		return ErrorBadArgumentParameter
	}

	// Copy command and MAC address into Directive
	d.Command = DirectiveCommand(b[1])
	mac := make(net.HardwareAddr, 6)
	copy(mac, b[2:])
	d.MAC = mac

	return nil
}

var (
	// Compile-time interface check
	_ Arg = &MACMaskArg{}
)

// A MACMaskArg is an argument to Command 2, MAC Mask List,
// (CommandMACMaskList) as described in AoEr11, Section 3.3.
type MACMaskArg struct {
	// Command specifies the MACMaskCommand carried in this argument.
	Command MACMaskCommand

	// Error, if not empty, specifies if an error occurred while processing
	// the Directives list.
	Error MACMaskError

	// DirCount specifies the number of Directive elements in this argument.
	DirCount uint8

	// Directives specifies a list of Directive elements, which are used to
	// interact with the MAC mask list.
	Directives []*Directive
}

// MarshalBinary allocates a byte slice containing the data from a MACMaskArg.
//
// If m.DirCount does not indicate the actual length of m.Directives, or
// a Directive is malformed, ErrorBadArgumentParameter is returned.
func (m *MACMaskArg) MarshalBinary() ([]byte, error) {
	// Must indicate correct number of directives
	if int(m.DirCount) != len(m.Directives) {
		return nil, ErrorBadArgumentParameter
	}

	// Allocate byte slice for argument and all directives
	b := make([]byte, macMaskArgLen+(directiveLen*m.DirCount))

	// 1 byte reserved

	b[1] = uint8(m.Command)
	b[2] = uint8(m.Error)
	b[3] = m.DirCount

	// Marshal each directive into binary and copy into byte slice
	// after argument
	n := 4
	for _, d := range m.Directives {
		db, err := d.MarshalBinary()
		if err != nil {
			return nil, err
		}

		copy(b[n:n+directiveLen], db)
		n += directiveLen
	}

	return b, nil
}

// UnmarshalBinary unmarshals a byte slice into a MACMaskArg.
//
// If the byte slice does not contain enough bytes to form a valid MACMaskArg,
// or a Directive is malformed, io.ErrUnexpectedEOF is returned.
//
// If byte 0 (reserved byte) is not empty, ErrorBadArgumentParameter is
// returned.
func (m *MACMaskArg) UnmarshalBinary(b []byte) error {
	// Must contain minimum length for argument
	if len(b) < macMaskArgLen {
		return io.ErrUnexpectedEOF
	}

	// 1 byte reserved
	if b[0] != 0 {
		return ErrorBadArgumentParameter
	}

	m.Command = MACMaskCommand(b[1])
	m.Error = MACMaskError(b[2])
	m.DirCount = b[3]

	// Must have exact number of bytes for directives with this count
	if len(b[4:]) != (directiveLen * int(m.DirCount)) {
		return io.ErrUnexpectedEOF
	}

	// Unmarshal each directive bytes and add to slice
	m.Directives = make([]*Directive, m.DirCount)
	for i := 0; i < int(m.DirCount); i++ {
		d := new(Directive)
		if err := d.UnmarshalBinary(b[4+(i*directiveLen) : 4+(i*directiveLen)+directiveLen]); err != nil {
			return err
		}
		m.Directives[i] = d
	}

	return nil
}
