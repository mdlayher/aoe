package aoe

import (
	"bytes"
	"io"
	"net"
	"reflect"
	"testing"
)

func TestReserveReleaseArgMarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		r    *ReserveReleaseArg
		b    []byte
		err  error
	}{
		{
			desc: "incorrect number of MAC addresses",
			r: &ReserveReleaseArg{
				NMACs: 1,
			},
			err: ErrorBadArgumentParameter,
		},
		{
			desc: "empty ReserveReleaseArg",
			r:    &ReserveReleaseArg{},
			b:    []byte{0, 0},
		},
		{
			desc: "MAC address too short",
			r: &ReserveReleaseArg{
				NMACs: 1,
				MACs: []net.HardwareAddr{
					{0xde, 0xad, 0xbe, 0xef, 0xde},
				},
			},
			err: ErrorBadArgumentParameter,
		},
		{
			desc: "MAC address too long",
			r: &ReserveReleaseArg{
				NMACs: 1,
				MACs: []net.HardwareAddr{
					{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad, 0xbe},
				},
			},
			err: ErrorBadArgumentParameter,
		},
		{
			desc: "one MAC address",
			r: &ReserveReleaseArg{
				NMACs: 1,
				MACs: []net.HardwareAddr{
					{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad},
				},
			},
			b: []byte{
				0, 1,
				0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
			},
		},
		{
			desc: "three MAC addresses",
			r: &ReserveReleaseArg{
				Command: 1,
				NMACs:   3,
				MACs: []net.HardwareAddr{
					{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad},
					{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
					{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
				},
			},
			b: []byte{
				1, 3,
				0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
				0x00, 0x11, 0x22, 0x33, 0x44, 0x55,
				0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
			},
		},
	}

	for i, tt := range tests {
		b, err := tt.r.MarshalBinary()
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

func TestReserveReleaseArgUnmarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		b    []byte
		r    *ReserveReleaseArg
		err  error
	}{
		{
			desc: "ReserveReleaseArg too short",
			b:    make([]byte, reserveReleaseArgLen-1),
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "no MAC addresses, but 1 indicated",
			b:    []byte{0, 1},
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "2 MAC addresses, but 1 indicated",
			b: []byte{
				0, 1,
				0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
				0x00, 0x11, 0x22, 0x33, 0x44, 0x55,
			},
			err: io.ErrUnexpectedEOF,
		},
		{
			desc: "empty ReserveReleaseArg",
			b:    []byte{0, 0},
			r: &ReserveReleaseArg{
				// Used to make reflect.DeepEqual happy
				MACs: make([]net.HardwareAddr, 0),
			},
		},
		{
			desc: "one MAC address",
			b: []byte{
				1, 1,
				0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
			},
			r: &ReserveReleaseArg{
				Command: 1,
				NMACs:   1,
				MACs: []net.HardwareAddr{
					{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad},
				},
			},
		},
		{
			desc: "three MAC addresses",
			b: []byte{
				1, 3,
				0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
				0x00, 0x11, 0x22, 0x33, 0x44, 0x55,
				0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
			},
			r: &ReserveReleaseArg{
				Command: 1,
				NMACs:   3,
				MACs: []net.HardwareAddr{
					{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad},
					{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
					{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
				},
			},
		},
	}

	for i, tt := range tests {
		r := new(ReserveReleaseArg)
		if err := r.UnmarshalBinary(tt.b); err != nil || tt.err != nil {
			if want, got := tt.err, err; want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.r, r; !reflect.DeepEqual(want, got) {
			t.Fatalf("[%02d] test %q, unexpected ReserveReleaseArg:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}
