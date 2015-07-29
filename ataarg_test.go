package aoe

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestATAArgMarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		a    *ATAArg
		b    []byte
	}{
		{
			desc: "empty ATAArg",
			a:    &ATAArg{},
			b:    make([]byte, ataArgLen),
		},
		{
			desc: "LBA48 extended, ATA device/head register flags set, LBA [0 1 2 3 4 5]",
			a: &ATAArg{
				FlagLBA48Extended:         true,
				FlagATADeviceHeadRegister: true,
				LBA: [6]uint8{0, 1, 2, 3, 4, 5},
			},
			b: []byte{0x50, 0, 0, 0, 0, 1, 2, 3, 4, 5, 0, 0},
		},
		{
			desc: "asynchronous, write flags set, LBA [5 4 3 2 1 0], data 'foo'",
			a: &ATAArg{
				FlagAsynchronous: true,
				FlagWrite:        true,
				LBA:              [6]uint8{5, 4, 3, 2, 1, 6},
				Data:             []byte("foo"),
			},
			b: []byte{0x03, 0, 0, 0, 5, 4, 3, 2, 1, 6, 0, 0, 'f', 'o', 'o'},
		},
		{
			desc: "err/feature 2, sector count 255, cmd status 4",
			a: &ATAArg{
				ErrFeature:  2,
				SectorCount: 255,
				CmdStatus:   4,
			},
			b: []byte{0x00, 2, 255, 4, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	}

	for i, tt := range tests {
		b, err := tt.a.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		if want, got := tt.b, b; !bytes.Equal(want, got) {
			t.Fatalf("[%02d] test %q, unexpected bytes:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}

func TestATAArgUnmarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		b    []byte
		a    *ATAArg
		err  error
	}{
		{
			desc: "ATAArg too short",
			b:    make([]byte, ataArgLen-1),
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "reserved bytes not empty",
			b:    []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1},
			err:  ErrorBadArgumentParameter,
		},
		{
			desc: "LBA [1 2 3 4 5 6], data 'foo'",
			b:    []byte{0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 0, 0, 'f', 'o', 'o'},
			a: &ATAArg{
				LBA:  [6]uint8{1, 2, 3, 4, 5, 6},
				Data: []byte("foo"),
			},
		},
		{
			desc: "LBA48 extended and ATA device/head register flags set, data '1'",
			b:    []byte{0x50, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			a: &ATAArg{
				FlagLBA48Extended:         true,
				FlagATADeviceHeadRegister: true,
				Data: []byte{1},
			},
		},
		{
			desc: "asynchronous and write flags set, err/feature 1, sector count 255, cmd status 2, LBA [6 6 6 6 6 6 6], data '1'",
			b:    []byte{0x03, 1, 255, 2, 6, 6, 6, 6, 6, 6, 0, 0, 1},
			a: &ATAArg{
				FlagAsynchronous: true,
				FlagWrite:        true,
				ErrFeature:       1,
				SectorCount:      255,
				CmdStatus:        2,
				LBA:              [6]uint8{6, 6, 6, 6, 6, 6},
				Data:             []byte{1},
			},
		},
	}

	for i, tt := range tests {
		a := new(ATAArg)
		if err := a.UnmarshalBinary(tt.b); err != nil || tt.err != nil {
			if want, got := tt.err, err; want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.a, a; !reflect.DeepEqual(want, got) {
			t.Fatalf("[%02d] test %q, unexpected ATAArg:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}
