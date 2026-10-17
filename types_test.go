package sfv

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBareItemWriteSFV - 各 BareItem の writeSFV が RFC 9651 Section 4.1.4〜4.1.11 どおりに出力するか確認する。
func TestBareItemWriteSFV(t *testing.T) {
	tests := []struct {
		name string
		in   BareItem
		want string
	}{
		{"integer zero", Integer(0), "0"},
		{"integer negative", Integer(-42), "-42"},
		{"integer max", Integer(maxIntegerValue), "999999999999999"},
		{"integer min", Integer(minIntegerValue), "-999999999999999"},

		{"decimal integral", Decimal(1), "1.0"},
		{"decimal zero", Decimal(0), "0.0"},
		{"decimal negative zero", Decimal(math.Copysign(0, -1)), "0.0"},
		{"decimal trailing zero trimmed", Decimal(0.5), "0.5"},
		{"decimal three digits", Decimal(-1.234), "-1.234"},
		{"decimal round odd half", Decimal(0.0015), "0.002"},
		{"decimal round even half", Decimal(0.0025), "0.002"},
		{"decimal round negative odd half", Decimal(-0.0015), "-0.002"},
		{"decimal round above half", Decimal(0.00251), "0.003"},
		{"decimal round below half", Decimal(1.0004999), "1.0"},
		{"decimal round to zero negative", Decimal(-0.0001), "0.0"},
		{"decimal carry into integer", Decimal(9.9995), "10.0"},
		{"decimal max", Decimal(999999999999.999), "999999999999.999"},
		{"decimal min", Decimal(-999999999999.999), "-999999999999.999"},

		{"string empty", String(""), `""`},
		{"string escape", String(`a"b\c`), `"a\"b\\c"`},
		{"string printable", String(" ~"), `" ~"`},

		{"token alpha", Token("foo123/456"), "foo123/456"},
		{"token star", Token("*"), "*"},
		{"token tchar", Token("a!#$%&'*+-.^_`|~:/"), "a!#$%&'*+-.^_`|~:/"},

		{"byte sequence", ByteSequence("hello"), ":aGVsbG8=:"},
		{"byte sequence empty", ByteSequence{}, "::"},
		{"byte sequence nil", ByteSequence(nil), "::"},

		{"boolean true", Boolean(true), "?1"},
		{"boolean false", Boolean(false), "?0"},

		{"date", Date(1659578233), "@1659578233"},
		{"date negative", Date(-62135596800), "@-62135596800"},

		{"display string ascii", DisplayString("abc"), `%"abc"`},
		{"display string non-ascii", DisplayString("Füü"), `%"F%c3%bc%c3%bc"`},
		{"display string percent", DisplayString("%"), `%"%25"`},
		{"display string dquote", DisplayString(`"`), `%"%22"`},
		{"display string backslash", DisplayString(`\`), `%"\"`},
		{"display string control", DisplayString("\x00\x1f\x7f"), `%"%00%1f%7f"`},
		{"display string emoji", DisplayString("😀"), `%"%f0%9f%98%80"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			if err := tt.in.writeSFV(&b); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := b.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBareItemWriteSFVError - 不正な BareItem が *SerializeError を返し、出力先に何も書き込まないことを確認する。
func TestBareItemWriteSFVError(t *testing.T) {
	tests := []struct {
		name    string
		in      BareItem
		wantErr error
	}{
		{"integer too big", Integer(maxIntegerValue + 1), ErrValueOutOfRange},
		{"integer too small", Integer(minIntegerValue - 1), ErrValueOutOfRange},

		{"decimal NaN", Decimal(math.NaN()), ErrValueOutOfRange},
		{"decimal +Inf", Decimal(math.Inf(1)), ErrValueOutOfRange},
		{"decimal -Inf", Decimal(math.Inf(-1)), ErrValueOutOfRange},
		{"decimal too big", Decimal(1000000000000.1), ErrValueOutOfRange},
		{"decimal too small", Decimal(-1000000000000.1), ErrValueOutOfRange},
		{"decimal too big after rounding", Decimal(999999999999.9995), ErrValueOutOfRange},
		{"decimal huge", Decimal(1e300), ErrValueOutOfRange},

		{"string control", String("a\nb"), ErrInvalidStringCharacter},
		{"string DEL", String("\x7f"), ErrInvalidStringCharacter},
		{"string latin-1", String("\xff"), ErrInvalidStringCharacter},
		{"string non-ascii", String("ü"), ErrInvalidStringCharacter},
		{"string beyond latin-1", String("日本"), ErrInvalidStringCharacter},

		{"token empty", Token(""), ErrInvalidTokenCharacter},
		{"token digit start", Token("1abc"), ErrInvalidTokenCharacter},
		{"token colon start", Token(":abc"), ErrInvalidTokenCharacter},
		{"token space", Token("a b"), ErrInvalidTokenCharacter},
		{"token non-ascii", Token("aü"), ErrInvalidTokenCharacter},

		{"date too big", Date(maxIntegerValue + 1), ErrValueOutOfRange},
		{"date too small", Date(minIntegerValue - 1), ErrValueOutOfRange},

		{"display string invalid utf-8", DisplayString("\xff"), ErrInvalidStringCharacter},
		{"display string surrogate", DisplayString("\xed\xa0\x80"), ErrInvalidStringCharacter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			err := tt.in.writeSFV(&b)

			var serr *SerializeError
			if !errors.As(err, &serr) {
				t.Fatalf("got %v, want *SerializeError", err)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got %v, want errors.Is(%v)", err, tt.wantErr)
			}
			if b.Len() != 0 {
				t.Errorf("partial output written: %q", b.String())
			}
		})
	}
}

// TestBareItemConformance - httpwg/structured-field-tests のうち Parameters を持たない Item のケースで、
// 期待値の BareItem を writeSFV した結果が canonical (無ければ raw) と一致するか確認する。
// シリアライザ本体が完成するまでの間、BareItem 単体の適合性を公式データで検証するためのもの。
func TestBareItemConformance(t *testing.T) {
	files := make([]string, 0, len(phase1Files)+len(phase2Files)+len(phase3Files)+4)
	files = append(files, phase1Files...)
	files = append(files, phase2Files...)
	files = append(files, phase3Files...)
	files = append(files,
		filepath.Join("serialisation-tests", "number.json"),
		filepath.Join("serialisation-tests", "string-generated.json"),
		filepath.Join("serialisation-tests", "token-generated.json"),
	)

	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			runBareItemTestFile(t, filepath.Join(testDir, name))
		})
	}
}

// runBareItemTestFile - 1 ファイル分のテストケースのうち、BareItem 単体で検証できるものを実行する。
func runBareItemTestFile(t *testing.T, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var cases []testCase
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&cases); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}

	// raw が無いケースはシリアライズ専用 (must_fail はシリアライズの失敗を意味する)。
	// raw があり must_fail のケースはパースの失敗を意味するので対象外。
	for _, tc := range cases {
		if tc.HeaderType != "item" || (tc.Raw != nil && tc.MustFail) {
			continue
		}

		t.Run(tc.Name, func(t *testing.T) {
			expected, err := decodeExpected(tc.HeaderType, tc.Expected)
			if err != nil {
				t.Fatalf("期待値デコード失敗: %v (expected=%s)", err, string(tc.Expected))
			}

			item := expected.(*Item)
			if item.Parameters.Len() != 0 {
				t.Skip("Parameters を持つ Item はシリアライザ本体で検証する")
			}

			var b strings.Builder
			err = item.Value.writeSFV(&b)

			if tc.MustFail {
				if err == nil {
					t.Errorf("must_fail=true だが成功した: got %q", b.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("想定外のシリアライズ失敗: %v", err)
			}

			wantParts := tc.Raw
			if tc.Canonical != nil {
				wantParts = tc.Canonical
			}
			if want := joinFieldLines(wantParts); b.String() != want {
				t.Errorf("got %q, want %q", b.String(), want)
			}
		})
	}
}
