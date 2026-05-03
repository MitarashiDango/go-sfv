package sfv

// BareItem - RFC 9651 §3.3 Bare Item を表す sealed interface。
type BareItem interface{ bareItem() }

// Integer - RFC 9651 §3.3.1 Integer (最大 15 桁、符号付き)。
type Integer int64

func (Integer) bareItem() {}

// Decimal - RFC 9651 §3.3.2 Decimal (整数部 ≤ 12 桁、小数部 ≤ 3 桁)。
type Decimal float64

func (Decimal) bareItem() {}

// String - RFC 9651 §3.3.3 String。
type String string

func (String) bareItem() {}

// Token - RFC 9651 §3.3.4 Token。
type Token string

func (Token) bareItem() {}

// ByteSequence - RFC 9651 §3.3.5 Byte Sequence。
type ByteSequence []byte

func (ByteSequence) bareItem() {}

// Boolean - RFC 9651 §3.3.6 Boolean。
type Boolean bool

func (Boolean) bareItem() {}

// Date - RFC 9651 §3.3.7 Date (Unix epoch 秒)。
type Date int64

func (Date) bareItem() {}

// DisplayString - RFC 9651 §3.3.8 Display String (UTF-8)。
type DisplayString string

func (DisplayString) bareItem() {}

// Parameters - RFC 9651 §3.1.2 Parameters。
type Parameters = OrderedMap[BareItem]

// ListMember - List の要素 (Item または InnerList) を表す sealed interface。
type ListMember interface{ listMember() }

// DictionaryMemberValue - Dictionary の値 (Item または InnerList) を表す sealed interface。
type DictionaryMemberValue interface{ dictMemberValue() }

// Item - RFC 9651 §3.3 Item (bare item + parameters)。
type Item struct {
	Value      BareItem
	Parameters Parameters
}

func (Item) listMember()      {}
func (Item) dictMemberValue() {}

// InnerList - RFC 9651 §3.1.1 Inner List。
type InnerList struct {
	Items      []*Item
	Parameters Parameters
}

func (InnerList) listMember()      {}
func (InnerList) dictMemberValue() {}

func (il InnerList) Len() int {
	return len(il.Items)
}

// List - RFC 9651 §3.1 List。
type List []ListMember

// Dictionary - RFC 9651 §3.2 Dictionary。
type Dictionary = OrderedMap[DictionaryMemberValue]
