// Package aoe implements an ATA over Ethernet server, as described in the
// AoEr11 specification.
//
// The AoEr11 specification can be found here:
// http://www.thebrantleycoilecompany.com/AoEr11.pdf.
package aoe

import (
	"encoding"

	"github.com/mdlayher/ethernet"
)

//go:generate stringer -output=string.go -type=Command,ConfigCommand,DirectiveCommand,Error,MACMaskCommand,MACMaskError,ReserveReleaseCommand

const (
	// Version is the ATA over Ethernet protocol version used by this package.
	Version uint8 = 1

	// EtherType is the registered EtherType for ATA over Ethernet, when the
	// protocol is encapsulated in a IEEE 802.3 Ethernet frame.
	EtherType ethernet.EtherType = 0x88a2

	// BroadcastMajor and BroadcastMinor are the wildcard values for the Major
	// and Minor values in a Header.
	BroadcastMajor uint16 = 0xffff
	BroadcastMinor uint8  = 0xff
)

// ResponseSender provides an interface which allows an AoE handler to
// construct and send a Header in response to a Request.
type ResponseSender interface {
	Send(*Header) (int, error)
}

// An Arg is an argument for a Command.  Different Arg implementations are
// used for different types of Commands.
type Arg interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

// An Error is an ATA over Ethernet error code, as described in AoEr11,
// Section 2.4.
//
// An error code is sent from a server to a client when an error occurs
// during a request.
type Error uint8

// Error returns the string representation of an Error code.
func (e Error) Error() string {
	return e.String()
}

const (
	// ErrorUnrecognizedCommandCode is returned when a server does not
	// understand the Command field in a Header.
	ErrorUnrecognizedCommandCode Error = 1

	// ErrorBadArgumentParameter is returned when an improper value exists
	// somewhere in an Arg field in a Header.
	ErrorBadArgumentParameter Error = 2

	// ErrorDeviceUnavailable is returned when a server can no longer accept
	// ATA commands.
	ErrorDeviceUnavailable Error = 3

	// ErrorConfigStringPresent is returned when a server cannot set a config
	// string, because one already exists.
	ErrorConfigStringPresent Error = 4

	// ErrorUnsupportedVersion is returned when a server does not understand
	// the Version number in a Header.
	ErrorUnsupportedVersion Error = 5

	// ErrorTargetIsReserved is returned when a command cannot be completed
	// because the target is reserved.
	ErrorTargetIsReserved Error = 6
)

// A Command is an ATA over Ethernet command.  Commands are used for operations
// such as issuing an ATA command, querying server configuration, reading or
// writing a MAC address access list, or reserving a device for client use.
type Command uint8

const (
	// CommandIssueATACommand is used to issue an ATA command to an attached
	// ATA device.
	CommandIssueATACommand Command = 0

	// CommandQueryConfigInformation is used to set or retrieve configuration
	// information to or from a server.
	CommandQueryConfigInformation Command = 1

	// CommandMACMaskList is used to read and manage a server access control
	// list based on client MAC addresses.
	CommandMACMaskList Command = 2

	// CommandReserveRelease is used to reserve or release an ATA over
	// Ethernet target for use by a set of clients.
	CommandReserveRelease Command = 3
)
