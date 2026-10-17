package sfv

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxIntegerValue  = 999_999_999_999_999
	minIntegerValue  = -999_999_999_999_999
	maxDecimalScaled = 999_999_999_999_999

	// maxDecimalIntegerDigits - RFC 9651 Section 3.3.2 Decimal の整数部の最大桁数。
	maxDecimalIntegerDigits = 12
	// decimalFractionDigits - RFC 9651 Section 3.3.2 Decimal の小数部の最大桁数。
	decimalFractionDigits = 3
)

// シリアライズ失敗の分類。writeSFV は常に *SerializeError を返し、
// その原因としてこれらを保持するため errors.Is で判別できる。
var (
	ErrValueOutOfRange        = errors.New("value is out of range")
	ErrInvalidStringCharacter = errors.New("invalid string character")
	ErrInvalidTokenCharacter  = errors.New("invalid token character")
)

// BareItem - RFC 9651 Section 3.3 Bare Item を表す sealed interface。
// Errata 8869 (Verified) により Item の型は以下の 8 種類: Integer, Decimal, String,
// Token, ByteSequence, Boolean, Date, DisplayString。
type BareItem interface {
	bareItem()

	// writeSFV - RFC 9651 Section 4.1.4〜4.1.11 に従って b へ書き出す。
	// 入力が不正な場合は *SerializeError を返し、b には何も書き込まない。
	writeSFV(b *strings.Builder) error
}

// Integer - RFC 9651 Section 3.3.1 Integer (-999,999,999,999,999〜999,999,999,999,999)。
type Integer int64

func (Integer) bareItem() {}

// writeSFV - RFC 9651 Section 4.1.4 Integer をシリアライズする。範囲外なら失敗する。
func (bi Integer) writeSFV(b *strings.Builder) error {
	if !isValidInteger(bi) {
		return wrapSerializeError(ErrValueOutOfRange, "integer is out of range")
	}

	b.WriteString(strconv.FormatInt(int64(bi), 10))

	return nil
}

// Decimal - RFC 9651 Section 3.3.2 Decimal (整数部 ≤ 12 桁、小数部 ≤ 3 桁)。
// float64 で保持するため、シリアライズ時はその値を往復可能な最短の 10 進表現
// (strconv.FormatFloat の 'f', -1) とみなして丸める。例えば Decimal(0.0025) は
// 2 進表現では 0.0025 よりわずかに大きいが、10 進の 0.0025 として扱い "0.002" になる。
type Decimal float64

func (Decimal) bareItem() {}

// writeSFV - RFC 9651 Section 4.1.5 Decimal をシリアライズする。
// 小数第 3 位へ round half to even で丸め、丸め後の整数部が 12 桁を超えれば失敗する。
// 小数部は末尾の 0 を除いて出力し、0 の場合は "0" を 1 桁出力する。
func (bi Decimal) writeSFV(b *strings.Builder) error {
	// NaN / ±Inf は "decimal number" ではない (step 1)。
	s := strconv.FormatFloat(float64(bi), 'f', -1, 64)
	if s == "NaN" || s == "+Inf" || s == "-Inf" {
		return wrapSerializeError(ErrValueOutOfRange, "decimal is not a finite number")
	}

	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")

	intPart, fracPart, _ := strings.Cut(s, ".")

	// 丸めで整数部の桁数が減ることはないので、丸め前に 12 桁を超えていれば失敗が確定する。
	// ここで弾いておくことで、以降の int64 変換が溢れないことも保証される。
	if len(intPart) > maxDecimalIntegerDigits {
		return wrapSerializeError(ErrValueOutOfRange, "decimal integer component is too long")
	}

	keep := fracPart
	rest := ""
	if len(fracPart) > decimalFractionDigits {
		keep, rest = fracPart[:decimalFractionDigits], fracPart[decimalFractionDigits:]
	}
	keep += strings.Repeat("0", decimalFractionDigits-len(keep))

	// scaled は入力の 1000 倍を小数第 3 位で切り捨てた値の絶対値 (最大 15 桁)。
	scaled, err := strconv.ParseInt(intPart+keep, 10, 64)
	if err != nil {
		return wrapSerializeError(ErrValueOutOfRange, "decimal cannot be represented")
	}

	// Step 2: 切り捨てた部分 rest を見て round half to even する。
	if roundUp(rest, scaled%2 == 1) {
		scaled++
	}

	// Step 3: 丸めで桁上がりし、整数部が 13 桁になった場合。
	if scaled > maxDecimalScaled {
		return wrapSerializeError(ErrValueOutOfRange, "decimal integer component is too long after rounding")
	}

	// Step 5: 丸めて 0 になった負数は "-0.0" ではなく "0.0" とする。
	if negative && scaled != 0 {
		b.WriteByte('-')
	}

	// Step 6
	b.WriteString(strconv.FormatInt(scaled/1000, 10))

	// Step 7
	b.WriteByte('.')

	// Step 8-9: 小数部は末尾の 0 を除いた 1〜3 桁。
	frac := strconv.FormatInt(scaled%1000+1000, 10)[1:]
	frac = strings.TrimRight(frac, "0")
	if frac == "" {
		frac = "0"
	}
	b.WriteString(frac)

	return nil
}

// roundUp - 切り捨てる 10 進数字列 rest に対し、round half to even で切り上げるべきかを返す。
// odd は切り捨て後の最終桁が奇数かどうか。
func roundUp(rest string, odd bool) bool {
	if rest == "" || rest[0] < '5' {
		return false
	}

	if rest[0] > '5' || strings.TrimRight(rest[1:], "0") != "" {
		return true
	}

	// ちょうど中間 (5000...) の場合は偶数側へ。
	return odd
}

// String - RFC 9651 Section 3.3.3 String (印字可能 ASCII %x20-7E のみ)。
type String string

func (String) bareItem() {}

// writeSFV - RFC 9651 Section 4.1.6 String をシリアライズする。
// %x20-7E 以外のバイト (制御文字、DEL、非 ASCII) を含む場合は失敗する。
func (bi String) writeSFV(b *strings.Builder) error {
	s := string(bi)

	// UTF-8 のマルチバイト文字も 0x80 以上のバイトとして検出されるよう、バイト単位で検査する。
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return wrapSerializeError(ErrInvalidStringCharacter, "string contains a non-ASCII or control character")
		}
	}

	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		if s[i] == '"' || s[i] == '\\' {
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	b.WriteByte('"')

	return nil
}

// Token - RFC 9651 Section 3.3.4 Token (先頭が ALPHA か "*"、以降は tchar / ":" / "/")。
type Token string

func (Token) bareItem() {}

// writeSFV - RFC 9651 Section 4.1.7 Token をシリアライズする。空文字列や不正な文字を含む場合は失敗する。
func (bi Token) writeSFV(b *strings.Builder) error {
	s := string(bi)

	if len(s) == 0 {
		return wrapSerializeError(ErrInvalidTokenCharacter, "token is empty")
	}

	if !isAlpha(s[0]) && s[0] != '*' {
		return wrapSerializeError(ErrInvalidTokenCharacter, "token must start with ALPHA or '*'")
	}

	for i := 1; i < len(s); i++ {
		if !isTchar(s[i]) && s[i] != ':' && s[i] != '/' {
			return wrapSerializeError(ErrInvalidTokenCharacter, "token contains an invalid character")
		}
	}

	b.WriteString(s)

	return nil
}

// ByteSequence - RFC 9651 Section 3.3.5 Byte Sequence。
type ByteSequence []byte

func (ByteSequence) bareItem() {}

// writeSFV - RFC 9651 Section 4.1.8 Byte Sequence をパディング付き base64 でシリアライズする。
// nil と空 slice はどちらも "::" になる。
func (bi ByteSequence) writeSFV(b *strings.Builder) error {
	b.WriteByte(':')
	b.WriteString(base64.StdEncoding.EncodeToString(bi))
	b.WriteByte(':')

	return nil
}

// Boolean - RFC 9651 Section 3.3.6 Boolean。
type Boolean bool

func (Boolean) bareItem() {}

// writeSFV - RFC 9651 Section 4.1.9 Boolean を "?1" / "?0" でシリアライズする。
// Dictionary / Parameters での値の省略は呼び出し側の責務。
func (bi Boolean) writeSFV(b *strings.Builder) error {
	if bi {
		b.WriteString("?1")
	} else {
		b.WriteString("?0")
	}

	return nil
}

// Date - RFC 9651 Section 3.3.7 Date (1970-01-01T00:00:00Z からの秒数、範囲は Integer と同じ)。
type Date int64

func (Date) bareItem() {}

// writeSFV - RFC 9651 Section 4.1.10 Date を "@" + Integer の形でシリアライズする。
func (bi Date) writeSFV(b *strings.Builder) error {
	// '@' を書く前に範囲を確認し、失敗時に b へ書き込まないようにする。
	if !isValidInteger(Integer(bi)) {
		return wrapSerializeError(ErrValueOutOfRange, "date is out of range")
	}

	b.WriteByte('@')

	return Integer(bi).writeSFV(b)
}

// DisplayString - RFC 9651 Section 3.3.8 Display String (Unicode スカラー値の列、Go では UTF-8 文字列)。
type DisplayString string

func (DisplayString) bareItem() {}

// writeSFV - RFC 9651 Section 4.1.11 Display String をシリアライズする。
// UTF-8 として不正な場合 (サロゲートを含む場合を含む) は失敗する。
// "%"、DQUOTE、%x00-1F、%x7F-FF のバイトは小文字 16 進の "%hh" にエンコードする。
func (bi DisplayString) writeSFV(b *strings.Builder) error {
	s := string(bi)

	if !utf8.ValidString(s) {
		return wrapSerializeError(ErrInvalidStringCharacter, "display string is not valid UTF-8")
	}

	b.WriteString(`%"`)
	// RFC はコードポイントではなく UTF-8 エンコード後のバイトを走査する。
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' || c == '"' || c <= 0x1f || c >= 0x7f {
			b.WriteByte('%')
			b.WriteByte(hexChars[c>>4])
			b.WriteByte(hexChars[c&0x0f])
		} else {
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')

	return nil
}

// Parameters - RFC 9651 Section 3.1.2 Parameters (キー順序を保持する Key → BareItem の map)。
type Parameters = OrderedMap[BareItem]

// ListMember - List の要素 (Item または InnerList) を表す sealed interface。
// marker method が値レシーバのため、値型とポインタ型のどちらも満たす。
type ListMember interface{ listMember() }

// DictionaryMemberValue - Dictionary の値 (Item または InnerList) を表す sealed interface。
// marker method が値レシーバのため、値型とポインタ型のどちらも満たす。
type DictionaryMemberValue interface{ dictMemberValue() }

// Item - RFC 9651 Section 3.3 Item (bare item + parameters)。
type Item struct {
	Value      BareItem
	Parameters Parameters
}

func (Item) listMember()      {}
func (Item) dictMemberValue() {}

// InnerList - RFC 9651 Section 3.1.1 Inner List (Item の配列 + Inner List 自身の parameters)。
type InnerList struct {
	Items      []*Item
	Parameters Parameters
}

func (InnerList) listMember()      {}
func (InnerList) dictMemberValue() {}

// Len - Inner List に含まれる Item の数を返す。
func (il InnerList) Len() int {
	return len(il.Items)
}

// List - RFC 9651 Section 3.1 List。
type List []ListMember

// Dictionary - RFC 9651 Section 3.2 Dictionary (キー順序を保持する Key → Item / InnerList の map)。
type Dictionary = OrderedMap[DictionaryMemberValue]
