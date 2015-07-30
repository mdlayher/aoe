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
