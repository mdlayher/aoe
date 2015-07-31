package aoe

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"testing"
)

func TestServeATA(t *testing.T) {
	abort := &ATAArg{
		CmdStatus:  ATACmdStatusErrStatus,
		ErrFeature: ATAErrAbort,
	}

	var tests = []struct {
		desc string
		r    *Header
		rs   io.ReadSeeker
		w    *ATAArg
		err  error
	}{
		{
			desc: "not CommandIssueATACommand",
			r: &Header{
				Command: CommandQueryConfigInformation,
			},
			err: ErrInvalidATARequest,
		},
		{
			desc: "not ATAArg",
			r: &Header{
				Command: CommandIssueATACommand,
				Arg:     &ConfigArg{},
			},
			err: ErrInvalidATARequest,
		},
		{
			desc: "ATA check power",
			r: &Header{
				Command: CommandIssueATACommand,
				Arg: &ATAArg{
					CmdStatus: ATACmdStatusCheckPower,
				},
			},
			w: &ATAArg{
				SectorCount: 0xff,
				CmdStatus:   ATACmdStatusReadyStatus,
			},
		},
		{
			desc: "ATA flush",
			r: &Header{
				Command: CommandIssueATACommand,
				Arg: &ATAArg{
					CmdStatus: ATACmdStatusFlush,
				},
			},
			w: &ATAArg{
				SectorCount: 0xff,
				CmdStatus:   ATACmdStatusReadyStatus,
			},
		},
		{
			desc: "ATA identify abort",
			r: &Header{
				Command: CommandIssueATACommand,
				Arg: &ATAArg{
					CmdStatus: ATACmdStatusIdentify,
					// Should be 1 for success
					SectorCount: 0,
				},
			},
			w: abort,
		},
		{
			desc: "ATA identify error",
			r: &Header{
				Command: CommandIssueATACommand,
				Arg: &ATAArg{
					CmdStatus: ATACmdStatusIdentify,
					// Should be 1 for success
					SectorCount: 1,
				},
			},
			rs:  &noopReadWriteSeeker{},
			err: ErrNotImplemented,
		},
		{
			desc: "ATA read 28-bit abort",
			r: &Header{
				Command: CommandIssueATACommand,
				Arg: &ATAArg{
					CmdStatus: ATACmdStatusRead28Bit,
					// Should be false for success
					FlagWrite: true,
				},
			},
			w: abort,
		},
		{
			desc: "ATA read 48-bit abort",
			r: &Header{
				Command: CommandIssueATACommand,
				Arg: &ATAArg{
					CmdStatus: ATACmdStatusRead48Bit,
					// Should be false for success
					FlagWrite: true,
				},
			},
			w: abort,
		},
		{
			desc: "ATA write 28-bit abort",
			r: &Header{
				Command: CommandIssueATACommand,
				Arg: &ATAArg{
					CmdStatus: ATACmdStatusWrite28Bit,
					// Should be true for success
					FlagWrite: false,
				},
			},
			w: abort,
		},
		{
			desc: "ATA write 48-bit abort",
			r: &Header{
				Command: CommandIssueATACommand,
				Arg: &ATAArg{
					CmdStatus: ATACmdStatusWrite48Bit,
					// Should be true for success
					FlagWrite: false,
				},
			},
			w: abort,
		},
		{
			desc: "ATA unknown command abort",
			r: &Header{
				Command: CommandIssueATACommand,
				Arg: &ATAArg{
					CmdStatus: 0xff,
				},
			},
			w: abort,
		},
	}

	for i, tt := range tests {
		w := &captureHeaderResponseSender{}

		if _, err := ServeATA(w, tt.r, tt.rs); err != nil || tt.err != nil {
			if want, got := tt.err, err; want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.w, w.h.Arg; !reflect.DeepEqual(want, got) {
			t.Fatalf("[%02d] test %q, unexpected ATAArg:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}

// captureHeaderResponseSender is a ResponseSender which captures the header
// passed to it in Send.
type captureHeaderResponseSender struct {
	h *Header
}

func (w *captureHeaderResponseSender) Send(h *Header) (int, error) {
	w.h = h
	return 0, nil
}

func Test_ataRead(t *testing.T) {
	// Error returned for error handling tests
	errFoo := errors.New("foo")

	var tests = []struct {
		desc string
		rarg *ATAArg
		rs   io.ReadSeeker
		warg *ATAArg
		err  error
	}{
		{
			desc: "non-ATA read command",
			rarg: &ATAArg{
				CmdStatus: 0,
			},
			err: errATAAbort,
		},
		{
			desc: "flagged as write",
			rarg: &ATAArg{
				FlagWrite: true,
				CmdStatus: ATACmdStatusRead28Bit,
			},
			err: errATAAbort,
		},
		{
			desc: "error during Seek",
			rarg: &ATAArg{
				CmdStatus: ATACmdStatusRead48Bit,
			},
			rs: &errSeeker{
				err: errFoo,
			},
			err: errFoo,
		},
		{
			desc: "error during Read",
			rarg: &ATAArg{
				CmdStatus: ATACmdStatusRead48Bit,
			},
			rs: &errReader{
				err: errFoo,
			},
			err: errFoo,
		},
		{
			desc: "read wrong number of bytes",
			rarg: &ATAArg{
				CmdStatus:   ATACmdStatusRead28Bit,
				SectorCount: 1,
			},
			rs: &nReader{
				n: sectorSize - 1,
			},
			err: errATAAbort,
		},
		{
			desc: "read OK",
			rarg: &ATAArg{
				CmdStatus:   ATACmdStatusRead48Bit,
				SectorCount: 2,
			},
			rs: &nReader{
				n: sectorSize * 2,
			},
			warg: &ATAArg{
				CmdStatus: ATACmdStatusReadyStatus,
				Data:      make([]byte, sectorSize*2),
			},
		},
	}

	for i, tt := range tests {
		warg, err := ataRead(tt.rarg, tt.rs)
		if err != nil || tt.err != nil {
			if want, got := tt.err, err; want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.warg, warg; !reflect.DeepEqual(want, got) {
			t.Fatalf("[%02d] test %q, unexpected ATAArg:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}

func Test_ataWrite(t *testing.T) {
	// Error returned for error handling tests
	errFoo := errors.New("foo")

	var tests = []struct {
		desc string
		rarg *ATAArg
		rs   io.ReadSeeker
		warg *ATAArg
		err  error
	}{
		{
			desc: "non-ATA write command",
			rarg: &ATAArg{
				CmdStatus: 0,
			},
			err: errATAAbort,
		},
		{
			desc: "not flagged as write",
			rarg: &ATAArg{
				FlagWrite: false,
				CmdStatus: ATACmdStatusWrite28Bit,
			},
			err: errATAAbort,
		},
		{
			desc: "data sector size and sector count mismatch",
			rarg: &ATAArg{
				FlagWrite:   true,
				CmdStatus:   ATACmdStatusWrite48Bit,
				SectorCount: 1,
				Data:        make([]byte, sectorSize-1),
			},
			err: errATAAbort,
		},
		{
			desc: "not io.ReadWriteSeeker",
			rarg: &ATAArg{
				FlagWrite: true,
				CmdStatus: ATACmdStatusWrite48Bit,
			},
			rs:  bytes.NewReader(nil),
			err: errATAAbort,
		},
		{
			desc: "error during Seek",
			rarg: &ATAArg{
				FlagWrite: true,
				CmdStatus: ATACmdStatusWrite48Bit,
			},
			rs: &errSeeker{
				err: errFoo,
			},
			err: errFoo,
		},
		{
			desc: "error during Write",
			rarg: &ATAArg{
				FlagWrite: true,
				CmdStatus: ATACmdStatusWrite48Bit,
			},
			rs: &errWriter{
				err: errFoo,
			},
			err: errFoo,
		},
		{
			desc: "wrong amount of data written",
			rarg: &ATAArg{
				FlagWrite:   true,
				CmdStatus:   ATACmdStatusWrite48Bit,
				SectorCount: 1,
				Data:        make([]byte, sectorSize),
			},
			rs: &nWriter{
				n: sectorSize - 1,
			},
			err: errATAAbort,
		},
		{
			desc: "write OK",
			rarg: &ATAArg{
				FlagWrite:   true,
				CmdStatus:   ATACmdStatusWrite28Bit,
				SectorCount: 2,
				Data:        make([]byte, sectorSize*2),
			},
			rs: &countWriter{},
			warg: &ATAArg{
				CmdStatus: ATACmdStatusReadyStatus,
			},
		},
	}

	for i, tt := range tests {
		warg, err := ataWrite(tt.rarg, tt.rs)
		if err != nil || tt.err != nil {
			if want, got := tt.err, err; want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.warg, warg; !reflect.DeepEqual(want, got) {
			t.Fatalf("[%02d] test %q, unexpected ATAArg:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}

func Test_ataIdentify(t *testing.T) {
	// Error returned by errIdentifier for error handling test
	errFoo := errors.New("foo")

	// Blank ATA device identifier since we don't do any introspection
	id := [512]byte{}

	var tests = []struct {
		desc string
		rarg *ATAArg
		rs   io.ReadSeeker
		warg *ATAArg
		err  error
	}{
		{
			desc: "non-ATA identify command",
			rarg: &ATAArg{
				CmdStatus: 0,
			},
			err: errATAAbort,
		},
		{
			desc: "too small sector count",
			rarg: &ATAArg{
				CmdStatus:   ATACmdStatusIdentify,
				SectorCount: 0,
			},
			err: errATAAbort,
		},
		{
			desc: "too large sector count",
			rarg: &ATAArg{
				CmdStatus:   ATACmdStatusIdentify,
				SectorCount: 2,
			},
			err: errATAAbort,
		},
		{
			desc: "io.ReadSeeker not Identifier",
			rarg: &ATAArg{
				CmdStatus:   ATACmdStatusIdentify,
				SectorCount: 1,
			},
			rs:  bytes.NewReader(nil),
			err: ErrNotImplemented,
		},
		{
			desc: "identify error",
			rarg: &ATAArg{
				CmdStatus:   ATACmdStatusIdentify,
				SectorCount: 1,
			},
			rs: &errIdentifier{
				err: errFoo,
			},
			err: errFoo,
		},
		{
			desc: "identify OK",
			rarg: &ATAArg{
				CmdStatus:   ATACmdStatusIdentify,
				SectorCount: 1,
			},
			rs: &errIdentifier{},
			warg: &ATAArg{
				CmdStatus: ATACmdStatusReadyStatus,
				Data:      id[:],
			},
		},
	}

	for i, tt := range tests {
		warg, err := ataIdentify(tt.rarg, tt.rs)
		if err != nil || tt.err != nil {
			if want, got := tt.err, err; want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.warg, warg; !reflect.DeepEqual(want, got) {
			t.Fatalf("[%02d] test %q, unexpected ATAArg:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}

func Test_calculateLBA(t *testing.T) {
	var tests = []struct {
		desc    string
		rlba    [6]uint8
		is48Bit bool
		lba     int64
	}{
		{
			desc: "zero LBA, 28-bit",
		},
		{
			desc:    "zero LBA, 48-bit",
			is48Bit: true,
		},
		{
			desc: "max LBA, 28-bit",
			rlba: [6]uint8{255, 255, 255, 255, 255, 255},
			lba:  268435455,
		},
		{
			desc:    "max LBA, 48-bit",
			rlba:    [6]uint8{255, 255, 255, 255, 255, 255},
			is48Bit: true,
			lba:     281474976710655,
		},
	}

	for i, tt := range tests {
		if want, got := tt.lba, calculateLBA(tt.rlba, tt.is48Bit); want != got {
			t.Fatalf("[%02d] test %q, unexpected LBA:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}

// nReader returns the value of its n field bytes whenever its Read method is called.
type nReader struct {
	n int
	noopReadWriteSeeker
}

func (r *nReader) Read(p []byte) (int, error) {
	return r.n, nil
}

// errReader returns the err field whenever its Read method is called.
type errReader struct {
	noopReadWriteSeeker
	err error
}

func (r *errReader) Read(p []byte) (int, error) {
	return 0, r.err
}

// errSeeker returns the err field whenever its Seek method is called.
type errSeeker struct {
	noopReadWriteSeeker
	err error
}

func (s *errSeeker) Seek(offset int64, whence int) (int64, error) {
	return 0, s.err
}

// errWriter returns the err field whenever its Write method is called.
type errWriter struct {
	noopReadWriteSeeker
	err error
}

func (w *errWriter) Write(p []byte) (int, error) {
	return 0, w.err
}

// countWriter returns len(p) bytes whenever its Write method is called.
type countWriter struct {
	noopReadWriteSeeker
}

func (w *countWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

// nWriter returns the value of its n field bytes whenever its Write method is called.
type nWriter struct {
	n int
	noopReadWriteSeeker
}

func (w *nWriter) Write(p []byte) (int, error) {
	return w.n, nil
}

// errIdentifier returns the err field whenever its Identify method is called.
type errIdentifier struct {
	noopReadWriteSeeker
	err error
}

func (i *errIdentifier) Identify() ([512]byte, error) {
	return [512]byte{}, i.err
}

// noopReadWriteSeeker is the no-op basis for other io.ReadWriteSeeker implementations.
type noopReadWriteSeeker struct{}

func (noopReadWriteSeeker) Read(p []byte) (int, error)                   { return 0, nil }
func (noopReadWriteSeeker) Write(p []byte) (int, error)                  { return 0, nil }
func (noopReadWriteSeeker) Seek(offset int64, whence int) (int64, error) { return 0, nil }
