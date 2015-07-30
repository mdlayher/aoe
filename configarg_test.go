package aoe

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestConfigArgMarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		c    *ConfigArg
		b    []byte
		err  error
	}{
		{
			desc: "command greater than 4-bit integer",
			c: &ConfigArg{
				Version: Version,
				Command: 0x1f,
			},
			err: ErrorBadArgumentParameter,
		},
		{
			desc: "config string length mismatch",
			c: &ConfigArg{
				Version:      Version,
				Command:      0x1,
				StringLength: 0,
				String:       []byte{0},
			},
			err: ErrorBadArgumentParameter,
		},
		{
			desc: "config string length 1025 (too long)",
			c: &ConfigArg{
				Version:      Version,
				Command:      0x1,
				StringLength: 1025,
				String:       make([]byte, 1025),
			},
			err: ErrorBadArgumentParameter,
		},
		{
			desc: "buffer count 10, firmware version 1, sector count 2, string 'foo'",
			c: &ConfigArg{
				BufferCount:     10,
				FirmwareVersion: 1,
				SectorCount:     2,
				Version:         Version,
				Command:         2,
				StringLength:    3,
				String:          []byte{'f', 'o', 'o'},
			},
			b: []byte{0, 10, 0, 1, 2, 0x12, 0, 3, 'f', 'o', 'o'},
		},
	}

	for i, tt := range tests {
		b, err := tt.c.MarshalBinary()
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

func TestConfigArgUnmarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		b    []byte
		c    *ConfigArg
		err  error
	}{
		{
			desc: "ConfigArg too short",
			b:    make([]byte, configArgLen-1),
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "buffer shorter than config string length",
			b:    []byte{0, 0, 0, 0, 0, 0x10, 0, 1},
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "config string length too long (greater than 1024)",
			b:    append([]byte{0, 0, 0, 0, 0, 0x10, 4, 1}, make([]byte, 1025)...),
			err:  ErrorBadArgumentParameter,
		},
		{
			desc: "buffer count 10, firmware version 1, sector count 2, string 'foo'",
			b:    []byte{0, 10, 0, 1, 2, 0x12, 0, 3, 'f', 'o', 'o'},
			c: &ConfigArg{
				BufferCount:     10,
				FirmwareVersion: 1,
				SectorCount:     2,
				Version:         Version,
				Command:         2,
				StringLength:    3,
				String:          []byte{'f', 'o', 'o'},
			},
		},
	}

	for i, tt := range tests {
		c := new(ConfigArg)
		if err := c.UnmarshalBinary(tt.b); err != nil || tt.err != nil {
			if want, got := tt.err, err; want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.c, c; !reflect.DeepEqual(want, got) {
			t.Fatalf("[%02d] test %q, unexpected ConfigArg:\n- want: %v\n-  got: %v",
				i, tt.desc, want, got)
		}
	}
}
