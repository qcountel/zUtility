package mem

import (
	"slices"
	"testing"
)

func TestParsePointer(t *testing.T) {
	type entry struct {
		name          string
		addr, offsets string

		expectFail,
		equalToPrevious bool
	}
	var prev *Pointer
	for _, tt := range []entry{
		{
			name:    "normal",
			addr:    "01921DF8",
			offsets: "18 38 D8 20 BE0",
		},
		{
			name:            "with 0x prefix",
			addr:            "0x01921DF8",
			offsets:         "0x18 38 0xD8 20 0xBE0",
			equalToPrevious: true,
		},
		{
			name:       "address fail",
			addr:       "0xx01921DF8",
			offsets:    "18 38 D8 20 BE0",
			expectFail: true,
		},
		{
			name:       "offsets fail",
			addr:       "01921DF8",
			offsets:    "00x18 xxx38 00xD8 0x20 0xxBE0",
			expectFail: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ptr, err := ParsePointer(tt.addr, tt.offsets)
			if err == nil && tt.expectFail {
				t.Fatalf("(%s) error is nil but expected fail; addr=%q,offsets=%q", tt.name, tt.addr, tt.offsets)
			}
			if err != nil && !tt.expectFail {
				t.Fatalf("(%s) error is %v but fail is not expected; addr=%q,offsets=%q", tt.name, err, tt.addr, tt.offsets)
			}
			if tt.equalToPrevious && (prev != nil && !pointersEqual(*prev, ptr)) {
				t.Fatalf("ptr %#v (%q) does not equals to %#v", ptr, tt.name, prev)
			}
			prev = &ptr
		})
	}
}

func pointersEqual(a, b Pointer) bool {
	return a.BaseAddress == b.BaseAddress &&
		slices.Equal(a.Offsets, b.Offsets)
}
