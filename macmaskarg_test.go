package aoe

import (
	"bytes"
	"io"
	"net"
	"reflect"
	"testing"
)

func TestMACMaskArgMarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		m    *MACMaskArg
		b    []byte
		err  error
	}{
		{
			desc: "directive length mismatch",
			m: &MACMaskArg{
				DirCount: 1,
			},
			err: ErrorBadArgumentParameter,
		},
		{
			desc: "malformed directive",
			m: &MACMaskArg{
				DirCount: 1,
				Directives: []*Directive{
					{
						Command: 0,
						MAC:     net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad, 0x00},
					},
				},
			},
			err: ErrorBadArgumentParameter,
		},
		{
			desc: "no directives",
			m:    &MACMaskArg{},
			b:    []byte{0, 0, 0, 0},
		},
		{
			desc: "one directive",
			m: &MACMaskArg{
				DirCount: 1,
				Directives: []*Directive{
					{
						Command: 1,
						MAC:     net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad},
					},
				},
			},
			b: []byte{
				0, 0, 0, 1,
				0, 1, 0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
			},
		},
		{
			desc: "three directives",
			m: &MACMaskArg{
				DirCount: 3,
				Directives: []*Directive{
					{
						Command: 1,
						MAC:     net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad},
					},
					{
						Command: 2,
						MAC:     net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
					},
					{
						Command: 3,
						MAC:     net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
					},
				},
			},
			b: []byte{
				0, 0, 0, 3,
				0, 1, 0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
				0, 2, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
				0, 3, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55,
			},
		},
	}

	for i, tt := range tests {
		b, err := tt.m.MarshalBinary()
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

func TestMACMaskArgUnmarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		b    []byte
		m    *MACMaskArg
		err  error
	}{
		{
			desc: "MACMaskArg too short",
			b:    make([]byte, macMaskArgLen-1),
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "reserved byte not empty",
			b:    []byte{255, 0, 0, 0},
			err:  ErrorBadArgumentParameter,
		},
		{
			desc: "too few directives for dircount",
			b:    []byte{0, 0, 0, 1},
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "bad directive",
			b: []byte{
				0, 0, 0, 1,
				1, 1, 0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
			},
			err: ErrorBadArgumentParameter,
		},
		{
			desc: "zero directives (with trailing bytes)",
			b: []byte{
				0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
			},
			m: &MACMaskArg{
				DirCount:   0,
				Directives: []*Directive{},
			},
		},
		{
			desc: "one directive",
			b: []byte{
				0, 0, 0, 1,
				0, 1, 0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
			},
			m: &MACMaskArg{
				DirCount: 1,
				Directives: []*Directive{
					{
						Command: 1,
						MAC:     net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad},
					},
				},
			},
		},
		{
			desc: "three directives",
			b: []byte{
				0, 0, 0, 3,
				0, 1, 0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
				0, 2, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
				0, 3, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55,
			},
			m: &MACMaskArg{
				DirCount: 3,
				Directives: []*Directive{
					{
						Command: 1,
						MAC:     net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad},
					},
					{
						Command: 2,
						MAC:     net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
					},
					{
						Command: 3,
						MAC:     net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
					},
				},
			},
		},
	}

	for i, tt := range tests {
		m := new(MACMaskArg)
		if err := m.UnmarshalBinary(tt.b); err != nil || tt.err != nil {
			if want, got := tt.err, err; want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.m, m; !reflect.DeepEqual(want, got) {
			t.Fatalf("[%02d] test %q, unexpected MACMaskArg:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}

func TestDirectiveMarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		d    *Directive
		b    []byte
		err  error
	}{
		{
			desc: "MAC address too short",
			d: &Directive{
				MAC: net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44},
			},
			err: ErrorBadArgumentParameter,
		},
		{
			desc: "MAC address too long",
			d: &Directive{
				MAC: net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66},
			},
			err: ErrorBadArgumentParameter,
		},
		{
			desc: "directive OK",
			d: &Directive{
				Command: 1,
				MAC:     net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
			},
			b: []byte{0, 1, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		},
	}

	for i, tt := range tests {
		b, err := tt.d.MarshalBinary()
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

func TestDirectiveUnmarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		b    []byte
		d    *Directive
		err  error
	}{
		{
			desc: "directive too short",
			b:    make([]byte, directiveLen-1),
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "directive too long",
			b:    make([]byte, directiveLen+1),
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "reserved byte not empty",
			b:    []byte{255, 0, 0, 0, 0, 0, 0, 0},
			err:  ErrorBadArgumentParameter,
		},
		{
			desc: "directive OK",
			b:    []byte{0, 1, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
			d: &Directive{
				Command: 1,
				MAC:     net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
			},
		},
	}

	for i, tt := range tests {
		d := new(Directive)
		if err := d.UnmarshalBinary(tt.b); err != nil || tt.err != nil {
			if want, got := tt.err, err; want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.d, d; !reflect.DeepEqual(want, got) {
			t.Fatalf("[%02d] test %q, unexpected Directive:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}
