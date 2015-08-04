package aoe

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// An ATACmdStatus is a value which indicates an ATA command or status.
type ATACmdStatus uint8

const (
	// ATAErrAbort indicates than an ATA command should be aborted.
	ATAErrAbort = 0x04

	// ATACmdStatus values recognized by ServeATA.
	ATACmdStatusErrStatus   ATACmdStatus = 0x01
	ATACmdStatusReadyStatus ATACmdStatus = 0x40
	ATACmdStatusCheckPower  ATACmdStatus = 0xe5
	ATACmdStatusFlush       ATACmdStatus = 0xe7
	ATACmdStatusIdentify    ATACmdStatus = 0xec
	ATACmdStatusRead28Bit   ATACmdStatus = 0x20
	ATACmdStatusRead48Bit   ATACmdStatus = 0x24
	ATACmdStatusWrite28Bit  ATACmdStatus = 0x30
	ATACmdStatusWrite48Bit  ATACmdStatus = 0x34

	// sectorSize is the required AoE sector size, as specified in AoEr11,
	// Section 3.
	sectorSize = 512
)

var (
	// ErrInvalidATARequest is returned when an invalid ATA request is rejected
	// by ServeATA.
	ErrInvalidATARequest = errors.New("invalid ATA request")

	// ErrNotImplemented is returned when functionality is not yet implemented.
	ErrNotImplemented = errors.New("not implemented")
)

// ServeATA replies to an AoE ATA request after performing the requested
// ATA operations on the io.ReadSeeker.  ServeATA can handle a variety of
// ATA requests, including reads, writes, and identification.
//
// ServeATA returns the number of bytes transmitted to a client, and any
// errors which occurred while processing a request.
//
// In order to make use of the full functionality provided by ServeATA, passing
// a block device to it as the io.ReadSeeker is recommended.  Package block
// can handle this functionality: https://github.com/mdlayher/block.
//
// If r.Command is not CommandIssueATACommand, or r.Arg is not a *ATAArg,
// ErrInvalidATARequest is returned.
//
// If ATA identification is requested, but rs does not implement Identifier,
// ErrNotImplemented is returned.  This behavior will change in the future,
// and Identifier implementations will be optional.
//
// If an ATA write is requested, but rs does not implement io.Writer, the ATA
// request will be aborted, but no error will be returned by ServeATA.
func ServeATA(w ResponseSender, r *Header, rs io.ReadSeeker) (int, error) {
	// Ensure request intends to issue an ATA command
	if r.Command != CommandIssueATACommand {
		return 0, ErrInvalidATARequest
	}
	arg, ok := r.Arg.(*ATAArg)
	if !ok {
		return 0, ErrInvalidATARequest
	}

	// Request to check device power mode or flush device writes
	// TODO(mdlayher): determine if these need to be different cases, because
	// they are no-op operations here
	if arg.CmdStatus == ATACmdStatusCheckPower || arg.CmdStatus == ATACmdStatusFlush {
		return w.Send(&Header{
			Arg: &ATAArg{
				// Device is active or idle
				SectorCount: 0xff,
				// Device is ready
				CmdStatus: ATACmdStatusReadyStatus,
			},
		})
	}

	// Handle other request types
	var warg *ATAArg
	var err error

	switch arg.CmdStatus {
	// Request to identify ATA device
	case ATACmdStatusIdentify:
		warg, err = ataIdentify(arg, rs)
	// Request for ATA read
	case ATACmdStatusRead28Bit, ATACmdStatusRead48Bit:
		warg, err = ataRead(arg, rs)
	// Request for ATA write
	case ATACmdStatusWrite28Bit, ATACmdStatusWrite48Bit:
		warg, err = ataWrite(arg, rs)
	// Unknown ATA command, abort
	default:
		// TODO(mdlayher): possibly expose SMART data when a *block.Device
		// is passed for rs
		err = errATAAbort
	}

	// Return non-abort errors
	if err != nil && err != errATAAbort {
		return 0, err
	}

	// If aborted, an ATAArg is returned stating that the command
	// was aborted
	if err == errATAAbort {
		warg = &ATAArg{
			CmdStatus:  ATACmdStatusErrStatus,
			ErrFeature: ATAErrAbort,
		}
	}

	// Reply to client; w handles Header field copying
	return w.Send(&Header{
		Arg: warg,
	})
}

// errATAAbort is returned when an ATA command is aborted due to incorrect
// request parameters.  It is a sentinel value used to indicate that a
// special abort response should be sent to a client, but it is not returned
// by ServeATA.
var errATAAbort = errors.New("ATA command aborted")

// An Identifier is an object which can return a 512 byte array containing
// ATA device identification information.
type Identifier interface {
	Identify() ([512]byte, error)
}

// ataIdentify performs an ATA identify request on rs using the argument
// values in r.
func ataIdentify(r *ATAArg, rs io.ReadSeeker) (*ATAArg, error) {
	// Only ATA device identify allowed here
	if r.CmdStatus != ATACmdStatusIdentify {
		return nil, errATAAbort
	}

	// Request must be for 1 sector (512 bytes)
	if r.SectorCount != 1 {
		return nil, errATAAbort
	}

	// If rs is an Identifier, request its identity directly
	ident, ok := rs.(Identifier)
	if !ok {
		// Currently no generic Identify implementation, as is done in
		// vblade.
		// TODO(mdlayher): add generic Identify implementation
		return nil, ErrNotImplemented
	}

	// Retrieve device identity information
	id, err := ident.Identify()
	if err != nil {
		return nil, err
	}

	return &ATAArg{
		CmdStatus: ATACmdStatusReadyStatus,
		Data:      id[:],
	}, nil
}

// ataRead performs an ATA 28-bit or 48-bit read request on rs using the
// argument values in r.
func ataRead(r *ATAArg, rs io.ReadSeeker) (*ATAArg, error) {
	// Only ATA reads allowed here
	if r.CmdStatus != ATACmdStatusRead28Bit && r.CmdStatus != ATACmdStatusRead48Bit {
		return nil, errATAAbort
	}

	// Read must not be flagged as a write
	if r.FlagWrite {
		return nil, errATAAbort
	}

	// Convert LBA to byte offset and seek to correct location
	offset := calculateLBA(r.LBA, r.FlagLBA48Extended) * sectorSize
	if _, err := rs.Seek(offset, os.SEEK_SET); err != nil {
		return nil, err
	}

	// Allocate buffer and read exact (sector count * sector size) bytes from
	// stream
	//
	// TODO(mdlayher): use r.Data instead of allocating?
	b := make([]byte, int(r.SectorCount)*sectorSize)
	n, err := rs.Read(b)
	if err != nil {
		return nil, err
	}

	// Verify sector count
	if sectors := n / sectorSize; sectors != int(r.SectorCount) {
		return nil, errATAAbort
	}

	return &ATAArg{
		CmdStatus: ATACmdStatusReadyStatus,
		Data:      b,
	}, nil
}

// ataWrite performs an ATA 28-bit or 48-bit write request on rs using the
// argument values in r.
func ataWrite(r *ATAArg, rs io.ReadSeeker) (*ATAArg, error) {
	// Only ATA writes allowed here
	if r.CmdStatus != ATACmdStatusWrite28Bit && r.CmdStatus != ATACmdStatusWrite48Bit {
		return nil, errATAAbort
	}

	// Write must be flagged as a write
	if !r.FlagWrite {
		return nil, errATAAbort
	}

	// Verify that request data and sector count match up
	if sectors := len(r.Data) / sectorSize; sectors != int(r.SectorCount) {
		return nil, errATAAbort
	}

	// Determine if io.ReadSeeker is also an io.Writer, and if a write is
	// requested
	rws, ok := rs.(io.ReadWriteSeeker)
	if !ok {
		// A write was requested, but the io.ReadSeeker is not an io.Writer
		return nil, errATAAbort
	}

	// TODO(mdlayher): implement asynchronous writes

	// Convert LBA to byte offset and seek to correct location
	offset := calculateLBA(r.LBA, r.FlagLBA48Extended) * sectorSize
	if _, err := rs.Seek(offset, os.SEEK_SET); err != nil {
		return nil, err
	}

	// Write data to stream
	n, err := rws.Write(r.Data)
	if err != nil {
		return nil, err
	}

	// Verify full sectors written to disk using sector count
	if sectors := n / sectorSize; sectors != int(r.SectorCount) {
		return nil, errATAAbort
	}

	return &ATAArg{
		CmdStatus: ATACmdStatusReadyStatus,
	}, nil
}

// calculateLBA calculates a logical block address from the LBA array
// and 48-bit flags from an ATAArg.
func calculateLBA(rlba [6]uint8, is48Bit bool) int64 {
	// Pad two bytes at the end to parse as uint64
	b := []byte{
		rlba[0],
		rlba[1],
		rlba[2],
		rlba[3],
		rlba[4],
		rlba[5],
		0,
		0,
	}
	lba := binary.LittleEndian.Uint64(b)

	// Mask off high bits to limit size to either 48 bit or 28 bit,
	// depending on is48Bit's value.
	if is48Bit {
		// 48-bit
		lba &= 0x0000ffffffffffff
	} else {
		// 28-bit
		lba &= 0x0fffffff
	}

	return int64(lba)
}
