// +build gofuzz

package aoe

func Fuzz(data []byte) int {
	h := new(Header)
	if err := h.UnmarshalBinary(data); err != nil {
		return 0
	}

	if _, err := h.MarshalBinary(); err != nil {
		panic(err)
	}

	return 1
}

/*
func ataIdentify() [512]byte {
	id := [256]uint16{}
	id[47] = 0x8000
	id[49] = 0x0200
	id[50] = 0x4000
	id[83] = 0x5400
	id[84] = 0x4000
	id[86] = 0x1400
	id[87] = 0x4000
	id[93] = 0x400b

	idb := [512]byte{}
	for i, j := 0, 0; i < len(idb); i, j = i+2, j+1 {
		binary.BigEndian.PutUint16(idb[i:i+2], id[j])
	}

	return idb
}
*/
