package aoe

import (
	"io"
)

const (
	// ataArgLen is the minimum required length for a valid ATAArg.
	//
	// 1 byte : flags
	//   0101 0011
	//    | |   ||
	//    | |   |+-- write flag
	//    | |   +--- asynchronous flag
	//    | +------- device/head register flag
	//    +--------- extended LBA48 flag
	// 1 byte : err/feature
	// 1 byte : sector count
	// 1 byte : cmd/status
	// 6 bytes: lba array
	// 2 bytes: reserved
	// N bytes: data
	ataArgLen = 1 + 1 + 1 + 1 + 6 + 2
)

var (
	// Compile-time interface check
	_ Arg = &ATAArg{}
)

// An ATAArg is an argument to Command 0, Issue ATA Command,
// (CommandIssueATACommand) as described in AoEr11, Section 3.1.
type ATAArg struct {
	// FlagLBA48Extended specifies if an LBA48 extended command is present in
	// this argument.  FlagATADeviceHeadRegister is not evaluated unless
	// FlagLBA48Extended is true.
	FlagLBA48Extended         bool
	FlagATADeviceHeadRegister bool

	// FlagAsynchronous specifies if a write request is to be done
	// asynchronously.
	FlagAsynchronous bool

	// FlagWrite specifies if data is to be written to a device.
	FlagWrite bool

	// TODO(mdlayher): document these fields
	ErrFeature  uint8
	SectorCount uint8
	CmdStatus   ATACmdStatus
	LBA         [6]uint8

	// Data is raw data to be transferred to and from a server.
	Data []byte
}

// MarshalBinary allocates a byte slice containing the data from an ATAArg.
//
// MarshalBinary never returns an error.
func (a *ATAArg) MarshalBinary() ([]byte, error) {
	// Allocate correct number of bytes for argument and data
	b := make([]byte, ataArgLen+len(a.Data))

	// Add bit flags at appropriate positions
	//
	// 0101 0011
	//  | |   ||
	//  | |   |+-- write flag
	//  | |   +--- asynchronous flag
	//  | +------- device/head register flag
	//  +--------- extended LBA48 flag
	var flags uint8
	if a.FlagLBA48Extended {
		flags |= (1 << 6)
	}
	if a.FlagATADeviceHeadRegister {
		flags |= (1 << 4)
	}
	if a.FlagAsynchronous {
		flags |= (1 << 1)
	}
	if a.FlagWrite {
		flags |= 1
	}
	b[0] = flags

	// Set other ATA data
	b[1] = a.ErrFeature
	b[2] = a.SectorCount
	b[3] = uint8(a.CmdStatus)
	b[4] = a.LBA[0]
	b[5] = a.LBA[1]
	b[6] = a.LBA[2]
	b[7] = a.LBA[3]
	b[8] = a.LBA[4]
	b[9] = a.LBA[5]

	// 2 bytes reserved space

	// Copy raw data after argument header
	copy(b[12:], a.Data)

	return b, nil
}

// UnmarshalBinary unmarshals a byte slice into an ATAArg.
//
// If the byte slice does not contain enough data to form a valid ATAArg,
// io.ErrUnexpectedEOF is returned.
//
// If bytes 10 and 11 are not zero (reserved bytes), ErrorBadArgumentParameter
// is returned.
func (a *ATAArg) UnmarshalBinary(b []byte) error {
	// Must contain minimum length for ATA argument
	if len(b) < ataArgLen {
		return io.ErrUnexpectedEOF
	}

	// 2 bytes reserved
	if b[10] != 0 || b[11] != 0 {
		return ErrorBadArgumentParameter
	}

	// Read bit flags at appropriate positions:
	//
	// 0101 0011
	//  | |   ||
	//  | |   |+-- write flag
	//  | |   +--- asynchronous flag
	//  | +------- device/head register flag
	//  +--------- extended LBA48 flag
	a.FlagLBA48Extended = (b[0] & 0x40) != 0
	a.FlagATADeviceHeadRegister = (b[0] & 0x10) != 0
	a.FlagAsynchronous = (b[0] & 0x02) != 0
	a.FlagWrite = (b[0] & 0x01) != 0

	// Read ATA data
	a.ErrFeature = b[1]
	a.SectorCount = b[2]
	a.CmdStatus = ATACmdStatus(b[3])
	a.LBA[0] = b[4]
	a.LBA[1] = b[5]
	a.LBA[2] = b[6]
	a.LBA[3] = b[7]
	a.LBA[4] = b[8]
	a.LBA[5] = b[9]

	// Copy raw data from ATA argument
	d := make([]byte, len(b[12:]))
	copy(d, b[12:])
	a.Data = d

	return nil
}
