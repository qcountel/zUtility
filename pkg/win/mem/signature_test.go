package mem

import (
	"bytes"
	"testing"
)

func TestMustParseSignature(t *testing.T) {
	type entry struct {
		name       string
		input      string
		expectData []byte
		expectMask string
		expectFail bool
	}
	for _, tt := range []entry{
		{
			name:       "normal hex",
			input:      "AE DE AD BE EF",
			expectData: []byte{0xAE, 0xDE, 0xAD, 0xBE, 0xEF},
			expectMask: "xxxxx",
		},
		{
			name:       "with wildcards",
			input:      "48 8B ? ? ? 05",
			expectData: []byte{0x48, 0x8B, 0x00, 0x00, 0x00, 0x05},
			expectMask: "xx???x",
		},
		{
			name:       "single char hex",
			input:      "A B C ?? D",
			expectData: []byte{0x0A, 0x0B, 0x0C, 0x00, 0x0D},
			expectMask: "xxx?x",
		},
		{
			name:       "invalid hex",
			input:      "GG WP",
			expectFail: true,
		},
		{
			name:       "wildcard variants",
			input:      "AA ? BB ?? CC",
			expectData: []byte{0xAA, 0x00, 0xBB, 0x00, 0xCC},
			expectMask: "x?x?x",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectFail {
				defer func() {
					if recover() == nil {
						t.Errorf("expected panic for input %q, but it didn't", tt.input)
					}
				}()
			}
			sig := MustParseSignature(tt.input)
			if !bytes.Equal(sig.Data, tt.expectData) {
				t.Errorf("Data mismatch: got %X, want %X", sig.Data, tt.expectData)
			}
			if sig.Mask != tt.expectMask {
				t.Errorf("Mask mismatch: got %q, want %q", sig.Mask, tt.expectMask)
			}
		})
	}
}

func TestMatchSignature(t *testing.T) {
	sig := MustParseSignature("48 8B ? ? 01")
	tests := []struct {
		name  string
		data  []byte
		match bool
	}{
		{
			name:  "exact match",
			data:  []byte{0x48, 0x8B, 0xAA, 0xBB, 0x01},
			match: true,
		},
		{
			name:  "match with different wildcards",
			data:  []byte{0x48, 0x8B, 0x00, 0xFF, 0x01},
			match: true,
		},
		{
			name:  "wrong start",
			data:  []byte{0x90, 0x8B, 0xAA, 0xBB, 0x01},
			match: false,
		},
		{
			name:  "wrong end",
			data:  []byte{0x48, 0x8B, 0xAA, 0xBB, 0x02},
			match: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if res := matchSignature(tt.data, sig); res != tt.match {
				t.Errorf("matchSignature='%v' but expected='%v'", res, tt.match)
			}
		})
	}
}

func TestSignatureEmpty(t *testing.T) {
	if !(Signature{}).Empty() {
		t.Error("empty signature marked as not empty")
	}
	if (Signature{Data: []byte{0x90}}).Empty() {
		t.Error("signature contains data but marked as empty")
	}
}
