package aoe

import (
	"bytes"
	"io"
	"net"
	"reflect"
	"testing"
)

func TestHeaderMarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		h    *Header
		b    []byte
		err  error
	}{
		{
			desc: "header version not 1",
			h: &Header{
				Version: 0x2,
			},
			err: ErrorUnsupportedVersion,
		},
		{
			desc: "nil Arg field",
			h: &Header{
				Version: Version,
			},
			err: ErrorBadArgumentParameter,
		},
		{
			desc: "error marshaling Arg field",
			h: &Header{
				Version: Version,
				Arg: &errArg{
					err: io.ErrUnexpectedEOF,
				},
			},
			err: io.ErrUnexpectedEOF,
		},
		{
			desc: "header OK, Version 1, Major 2, Minor 3",
			h: &Header{
				Version: Version,
				Major:   2,
				Minor:   3,
				Arg:     &noopArg{},
			},
			b: []byte{0x10, 0, 0, 2, 3, 0, 0, 0, 0, 0},
		},
		{
			desc: "header OK, Version 1, FlagResponse true, FlagError true, Error 1",
			h: &Header{
				Version:      Version,
				FlagResponse: true,
				FlagError:    true,
				Error:        1,
				Arg:          &noopArg{},
			},
			b: []byte{0x1c, 1, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	}

	for i, tt := range tests {
		b, err := tt.h.MarshalBinary()
		if err != nil || tt.err != nil {
			if want, got := tt.err, err; want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.b, b; !bytes.Equal(want, got) {
			t.Fatalf("[%02d] test %q, unexpected bytes:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}

func TestHeaderUnmarshalAndMarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		b    []byte
	}{
		{
			desc: "go-fuzz crasher: ConfigArg.Version accepted in unmarshal, but rejected in marshal",
			b:    []byte("\x100000\x010000000000\x00\x00"),
		},
		{
			desc: "header with CommandIssueATACommand, ATAArg OK",
			b: []byte{
				0x10, 0, 0, 1, 2, 0, 0, 0, 0, 10,
				0x53, 1, 2, 3, 6, 6, 6, 6, 6, 6, 0, 0, 'f', 'o', 'o',
			},
		},
		{
			desc: "header with CommandQueryConfigInformation, ConfigArg OK",
			b: []byte{
				0x10, 0, 0, 1, 2, 1, 0, 0, 0, 10,
				0, 10, 0, 1, 2, 0x11, 0, 3, 'f', 'o', 'o',
			},
		},
		{
			desc: "header with CommandMACMaskList, MACMaskArg OK",
			b: []byte{
				0x10, 0, 0, 1, 2, 2, 0, 0, 0, 10,
				0, 0, 0, 1,
				0, 1, 0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
			},
		},
		{
			desc: "header with CommandReserveRelease, ReserveReleaseArg OK",
			b: []byte{
				0x10, 0, 0, 1, 2, 3, 0, 0, 0, 10,
				0, 1,
				0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
			},
		},
	}

	for i, tt := range tests {
		h := new(Header)
		if err := h.UnmarshalBinary(tt.b); err != nil {
			t.Fatalf("[%02d] unmarshal test %q, %v", i, tt.desc, err)
		}

		b, err := h.MarshalBinary()
		if err != nil {
			t.Fatalf("[%02d] marshal test %q, %v", i, tt.desc, err)
		}

		if want, got := tt.b, b; !bytes.Equal(want, got) {
			t.Fatalf("[%02d] test %q, unexpected bytes:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}

func TestHeaderUnmarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		b    []byte
		h    *Header
		err  error
	}{
		{
			desc: "header too short",
			b:    make([]byte, headerLen-1),
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "header version not 1",
			b:    []byte{0x20, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			err:  ErrorUnsupportedVersion,
		},
		{
			desc: "unknown command",
			b:    []byte{0x10, 0, 0, 0, 0, 4, 0, 0, 0, 0},
			err:  ErrorUnrecognizedCommandCode,
		},
		{
			desc: "header with CommandIssueATACommand, ATAArg unexpected EOF",
			b:    []byte{0x10, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "header with CommandIssueATACommand, ATAArg OK",
			b: []byte{
				0x10, 0, 0, 1, 2, 0, 0, 0, 0, 10,
				0, 1, 2, 3, 6, 6, 6, 6, 6, 6, 0, 0, 'f', 'o', 'o',
			},
			h: &Header{
				Version: Version,
				Major:   1,
				Minor:   2,
				Command: CommandIssueATACommand,
				Tag:     [4]byte{0, 0, 0, 10},
				Arg: &ATAArg{
					ErrFeature:  1,
					SectorCount: 2,
					CmdStatus:   3,
					LBA:         [6]uint8{6, 6, 6, 6, 6, 6},
					Data:        []byte("foo"),
				},
			},
		},
		{
			desc: "header with CommandQueryConfigInformation, ConfigArg unexpected EOF",
			b:    []byte{0x10, 0, 0, 0, 0, 1, 0, 0, 0, 0},
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "header with CommandQueryConfigInformation, ConfigArg OK",
			b: []byte{
				0x10, 0, 0, 1, 2, 1, 0, 0, 0, 10,
				0, 10, 0, 1, 2, 0x11, 0, 3, 'f', 'o', 'o',
			},
			h: &Header{
				Version: Version,
				Major:   1,
				Minor:   2,
				Command: CommandQueryConfigInformation,
				Tag:     [4]byte{0, 0, 0, 10},
				Arg: &ConfigArg{
					BufferCount:     10,
					FirmwareVersion: 1,
					SectorCount:     2,
					Version:         Version,
					Command:         1,
					StringLength:    3,
					String:          []byte("foo"),
				},
			},
		},
		{
			desc: "header with CommandMACMaskList, MACMaskArg unexpected EOF",
			b:    []byte{0x10, 0, 0, 0, 0, 2, 0, 0, 0, 0},
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "header with CommandMACMaskList, MACMaskArg OK",
			b: []byte{
				0x10, 0, 0, 1, 2, 2, 0, 0, 0, 10,
				0, 0, 0, 1,
				0, 1, 0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
			},
			h: &Header{
				Version: Version,
				Major:   1,
				Minor:   2,
				Command: CommandMACMaskList,
				Tag:     [4]byte{0, 0, 0, 10},
				Arg: &MACMaskArg{
					DirCount: 1,
					Directives: []*Directive{
						{
							Command: 1,
							MAC:     net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad},
						},
					},
				},
			},
		},
		{
			desc: "header with CommandReserveRelease, ReserveReleaseArg unexpected EOF",
			b:    []byte{0x10, 0, 0, 0, 0, 3, 0, 0, 0, 0},
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "header with CommandReserveRelease, ReserveReleaseArg OK",
			b: []byte{
				0x10, 0, 0, 1, 2, 3, 0, 0, 0, 10,
				0, 1,
				0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
			},
			h: &Header{
				Version: Version,
				Major:   1,
				Minor:   2,
				Command: CommandReserveRelease,
				Tag:     [4]byte{0, 0, 0, 10},
				Arg: &ReserveReleaseArg{
					NMACs: 1,
					MACs: []net.HardwareAddr{
						{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad},
					},
				},
			},
		},
	}

	for i, tt := range tests {
		h := new(Header)
		if err := h.UnmarshalBinary(tt.b); err != nil || tt.err != nil {
			if want, got := tt.err, err; want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.h, h; !reflect.DeepEqual(want, got) {
			t.Fatalf("[%02d] test %q, unexpected Header:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}

type errArg struct {
	err error
	noopArg
}

func (a errArg) MarshalBinary() ([]byte, error) {
	return nil, a.err
}

type noopArg struct{}

func (noopArg) MarshalBinary() ([]byte, error) {
	return nil, nil
}
func (noopArg) UnmarshalBinary(b []byte) error {
	return nil
}
