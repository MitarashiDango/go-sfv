package sfv

import (
	"bytes"
	"testing"
)

// TestParseByteSequencePadding - RFC 9651 Section 4.2.7 の base64 パディングの扱いを確認する。
// "=" パディングの欠落と非ゼロの pad bits では失敗させず (SHOULD NOT)、それ以外の不正な base64 は失敗させる。
func TestParseByteSequencePadding(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []byte
		wantErr bool
	}{
		{"padded", ":aGVsbG8=:", []byte("hello"), false},
		{"missing one pad", ":aGVsbG8:", []byte("hello"), false},
		{"missing two pads", ":aGVsbA:", []byte("hell"), false},
		{"partial padding", ":aGVsbA=:", []byte("hell"), false},
		{"no padding needed", ":aGVs:", []byte("hel"), false},
		{"non-zero pad bits", ":iZ==:", []byte{0x89}, false},
		{"non-zero pad bits unpadded", ":iZ:", []byte{0x89}, false},

		{"excess padding", ":aGVsbG8==:", nil, true},
		{"padding only", ":====:", nil, true},
		{"single trailing char", ":aGVsb:", nil, true},
		{"padding in middle", ":a=GVsbG8=:", nil, true},
		{"padding at beginning", ":=aGVsbG8=:", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := ParseItem([]string{tt.raw})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %#v", item.Value)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, ok := item.Value.(ByteSequence)
			if !ok {
				t.Fatalf("got %T, want ByteSequence", item.Value)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("got %x, want %x", []byte(got), tt.want)
			}
		})
	}
}
