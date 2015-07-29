package aoe

import (
	"io"
	"net"
)

// A ReserveReleaseCommand is a subcommand used with a ReserveReleaseArg, as
// described in AoEr11, Section 3.4.
type ReserveReleaseCommand uint8

const (
	// ReserveReleaseCommandRead reads a server's reserve list.
	ReserveReleaseCommandRead ReserveReleaseCommand = 0

	// ReserveReleaseCommandSet attempts to modify a server's reserve list,
	// but only if the server's reserve list is empty or the source address of
	// the command is in the reserve list.
	ReserveReleaseCommandSet ReserveReleaseCommand = 1

	// ReserveReleaseCommandForceSet forcibly modify's a server's reserve list.
	ReserveReleaseCommandForceSet ReserveReleaseCommand = 2
)

var (
	// Compile-time interface check
	_ Arg = &ReserveReleaseArg{}
)

const (
	// reserveReleaseArgLen is the minimum required length for a
	// ReserveReleaseArg.
	//
	//   1 byte : reserve/release command
	//   1 byte : number of MAC addresses
	// 6*N bytes: MAC addresses
	reserveReleaseArgLen = 2
)

// A ReserveReleaseArg is an argument to Command 3, Reserve/Release
// (CommandReserveRelease) as described in AoEr11, Section 3.4.
type ReserveReleaseArg struct {
	// Command specifies the ReserveReleaseCommand carried in this argument.
	Command ReserveReleaseCommand

	// NMACs specifies the number of hardware address elements in this
	// argument.
	NMACs uint8

	// MACs specifies a list of hardware addresses, which are used to interact
	// with a server's reserve list.
	MACs []net.HardwareAddr
}

// MarshalBinary allocates a byte slice containing the data from a
// ReserveReleaseArg.
//
// If r.NMACs does not indicate the actual length of r.MACs, or one or more
// hardware addresses are not exactly 6 bytes in length,
// ErrorBadArgumentParameter is returned.
func (r *ReserveReleaseArg) MarshalBinary() ([]byte, error) {
	// Must indicate correct number of hardware addresses
	if int(r.NMACs) != len(r.MACs) {
		return nil, ErrorBadArgumentParameter
	}

	// Allocate byte slice for argument and hardware addresses
	b := make([]byte, reserveReleaseArgLen+(r.NMACs*6))

	b[0] = uint8(r.Command)
	b[1] = uint8(r.NMACs)

	// Copy each hardware address into byte slice, after verifying exactly
	// 6 bytes in length
	n := 2
	for _, m := range r.MACs {
		if len(m) != 6 {
			return nil, ErrorBadArgumentParameter
		}

		copy(b[n:n+6], m)
		n += 6
	}

	return b, nil
}

// UnmarshalBinary unmarshals a byte slice into a ReserveReleaseArg.
//
// If the byte slice does not contain enough bytes to form a valid
// ReserveReleaseArg, or a hardware address is malformed, io.ErrUnexpectedEOF
// is returned.
func (r *ReserveReleaseArg) UnmarshalBinary(b []byte) error {
	// Must contain minimum length for argument
	if len(b) < reserveReleaseArgLen {
		return io.ErrUnexpectedEOF
	}

	r.Command = ReserveReleaseCommand(b[0])
	r.NMACs = b[1]

	// Must have exact number of bytes for hardware addresses with
	// this count
	if len(b[2:]) != (6 * int(r.NMACs)) {
		return io.ErrUnexpectedEOF
	}

	// Copy each hardware address into slice
	r.MACs = make([]net.HardwareAddr, r.NMACs)
	for i := 0; i < int(r.NMACs); i++ {
		m := make(net.HardwareAddr, 6)
		copy(m, b[2+(i*6):2+(i*6)+6])
		r.MACs[i] = m
	}

	return nil
}
