package sfv

import (
	"encoding/base64"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

type parser struct {
	input string
	pos   int
}

func (p *parser) peek() (byte, bool) {
	if p.pos >= len(p.input) {
		return 0, false
	}

	return p.input[p.pos], true
}

func (p *parser) advance() (byte, bool) {
	if p.pos >= len(p.input) {
		return 0, false
	}

	b := p.input[p.pos]
	p.pos++

	return b, true
}

func (p *parser) discardLeadingSP() {
	for p.pos < len(p.input) && isSP(p.input[p.pos]) {
		p.pos++
	}
}

func (p *parser) discardLeadingOWS() {
	for p.pos < len(p.input) && isOWS(p.input[p.pos]) {
		p.pos++
	}
}

func (p *parser) isEmpty() bool {
	return p.pos >= len(p.input)
}

// parseList - RFC 9651 §4.2.1 List をパースする。
func (p *parser) parseList() (List, error) {
	result := make(List, 0, 10)

	for !p.isEmpty() {
		ch, ok := p.peek()
		if !ok {
			return result, nil
		}

		switch ch {
		case '(':
			innerList, err := p.parseInnerList()
			if err != nil {
				return nil, err
			}

			result = append(result, innerList)

		default:
			item, err := p.parseItem()
			if err != nil {
				return nil, err
			}

			result = append(result, item)
		}

		p.discardLeadingOWS()

		ch, ok = p.advance()
		if !ok {
			return result, nil
		}

		if ch != ',' {
			return nil, p.errAt("expected ',' or end of input after list member")
		}

		p.discardLeadingOWS()

		if p.isEmpty() {
			return nil, p.errAt("trailing comma in list")
		}
	}

	return result, nil
}

// parseInnerList - RFC 9651 §4.2.1.2 Inner List をパースする。
func (p *parser) parseInnerList() (*InnerList, error) {
	result := new(InnerList)

	ch, ok := p.advance()
	if !ok {
		return nil, p.errAt("expected '(' at start of inner list")
	} else if ch != '(' {
		return nil, p.errAt("expected '(' at start of inner list")
	}

	for !p.isEmpty() {
		ch, ok := p.peek()
		if !ok {
			break
		}

		switch {
		case ch == ')':
			p.advance()
			params, err := p.parseParameters()
			if err != nil {
				return nil, err
			}
			result.Parameters = *params
			return result, nil

		case isSP(ch):
			p.discardLeadingSP()

		default:
			item, err := p.parseItem()
			if err != nil {
				return nil, err
			}

			result.Items = append(result.Items, item)

			// RFC 9651 §4.2.1.2 step 3.5: Item 直後は SP か ')' でなければ fail。
			next, ok := p.peek()
			if !ok {
				break
			}

			if !isSP(next) && next != ')' {
				return nil, p.errAt("expected ' ' or ')' after inner list member")
			}
		}
	}

	return nil, p.errAt("unexpected end of input in inner list")
}

// parseDictionary - RFC 9651 §4.2.2 Dictionary をパースする。
func (p *parser) parseDictionary() (*Dictionary, error) {
	result := new(Dictionary)

	for !p.isEmpty() {
		key, value, err := p.parseDictionaryMember()
		if err != nil {
			return nil, err
		}

		result.Set(key, value)

		p.discardLeadingOWS()

		ch, ok := p.advance()
		if !ok {
			return result, nil
		}

		if ch != ',' {
			return nil, p.errAt("expected ',' or end of input after dictionary member")
		}

		p.discardLeadingOWS()

		if p.isEmpty() {
			return nil, p.errAt("trailing comma in dictionary")
		}
	}

	return result, nil
}

func (p *parser) parseDictionaryMember() (string, DictionaryMemberValue, error) {
	keyName, err := p.parseKey()
	if err != nil {
		return "", nil, err
	}

	ch, ok := p.peek()
	if ok && ch == '=' {
		p.advance()

		memberValue, err := p.parseDictionaryMemberValue()
		if err != nil {
			return "", nil, err
		}

		return keyName, memberValue, nil
	}

	// RFC 9651 §4.2.2 step 2.3: '=' 無しは value=Boolean(true) で常に Parameters をパース。
	params, err := p.parseParameters()
	if err != nil {
		return "", nil, err
	}

	return keyName, &Item{
		Value:      Boolean(true),
		Parameters: *params,
	}, nil
}

func (p *parser) parseDictionaryMemberValue() (DictionaryMemberValue, error) {
	ch, ok := p.peek()
	if !ok {
		return nil, p.errAt("unexpected end of input in dictionary member value")
	}

	switch ch {
	case '(':
		innerList, err := p.parseInnerList()
		if err != nil {
			return nil, err
		}

		return innerList, nil

	default:
		item, err := p.parseItem()
		if err != nil {
			return nil, err
		}

		return item, nil
	}
}

// parseItem - RFC 9651 §4.2.3 Item (bare item + parameters) をパースする。
func (p *parser) parseItem() (*Item, error) {
	bareItem, err := p.parseBareItem()
	if err != nil {
		return nil, err
	}

	params, err := p.parseParameters()
	if err != nil {
		return nil, err
	}

	return &Item{
		Value:      bareItem,
		Parameters: *params,
	}, nil
}

// parseParameters - RFC 9651 §4.2.3.2 Parameters をパースする。先頭が ';' でなければ空の Parameters を返す (エラーではない)。
func (p *parser) parseParameters() (*Parameters, error) {
	result := new(Parameters)

	for !p.isEmpty() {
		ch, ok := p.peek()
		if !ok || ch != ';' {
			return result, nil
		}
		p.advance()

		p.discardLeadingSP()

		key, err := p.parseKey()
		if err != nil {
			return nil, err
		}

		ch, ok = p.peek()
		if ok && ch == '=' {
			p.advance()

			value, err := p.parseBareItem()
			if err != nil {
				return nil, err
			}

			result.Set(key, value)
		} else {
			result.Set(key, Boolean(true))
		}

		ch, ok = p.peek()
		if !ok || ch != ';' {
			return result, nil
		}
	}

	return result, nil
}

// parseKey - RFC 9651 §4.2.3.3 Key をパースする。
func (p *parser) parseKey() (string, error) {
	ch, ok := p.advance()
	if !ok {
		return "", p.errAt("expected key character")
	}

	if !isKeyStart(ch) {
		return "", p.errAt("invalid key start character")
	}

	var keyBuilder strings.Builder
	keyBuilder.Grow(10)
	keyBuilder.WriteByte(ch)

	for !p.isEmpty() {
		ch, ok := p.peek()
		if !ok {
			return keyBuilder.String(), nil
		}

		if !isKeyChar(ch) {
			return keyBuilder.String(), nil
		}

		keyBuilder.WriteByte(ch)
		p.advance()
	}

	return keyBuilder.String(), nil
}

// parseBareItem - RFC 9651 §4.2.3.1 Bare Item を先頭文字で 8 種にディスパッチする。
func (p *parser) parseBareItem() (BareItem, error) {
	ch, ok := p.peek()
	if !ok {
		return nil, p.errAt("unexpected end of input when parsing bare item")
	}

	switch {
	case ch == '-', isDigit(ch):
		return p.parseIntegerOrDecimal()

	case ch == '"':
		return p.parseString()

	case ch == '*', isAlpha(ch):
		return p.parseToken()

	case ch == ':':
		return p.parseBinarySequence()

	case ch == '?':
		return p.parseBoolean()

	case ch == '@':
		return p.parseDate()

	case ch == '%':
		return p.parseDisplayString()

	default:
		return nil, p.errAt("invalid bare item")
	}
}

// parseIntegerOrDecimal - RFC 9651 §4.2.4 Integer または Decimal をパースする。
// アルゴリズムは仕様の step 番号に対応:
//   - step 4: 先頭の '-' を消費して符号を決める
//   - step 5/6: 符号後の最初の文字が DIGIT でなければ fail
//   - step 7: DIGIT を読み続け、'.' で integer → decimal に遷移。長さ制限を逐次チェック
//   - step 9.1: decimal は '.' で終わってはならない
func (p *parser) parseIntegerOrDecimal() (BareItem, error) {
	// Step 4: optional '-'
	isNegative := false
	if ch, ok := p.peek(); ok && ch == '-' {
		p.advance()
		isNegative = true
	}

	// Step 5: 符号のみで終了 / Step 6: 最初の文字は DIGIT 必須
	ch, ok := p.peek()
	if !ok {
		return nil, p.errAt("empty integer")
	}
	if !isDigit(ch) {
		return nil, p.errAt("integer must start with a digit")
	}

	// numBuf は RFC でいう input_number (符号を含まない数値部の文字列)。
	var numBuf strings.Builder
	numBuf.Grow(16)

	isDecimal := false
	fracDigits := 0

	// Step 7
	for !p.isEmpty() {
		ch, _ := p.peek()

		// Step 7.5 相当: DIGIT でも (まだ未出現の) '.' でもない文字に当たったら確定。
		canContinue := isDigit(ch) || (ch == '.' && !isDecimal)
		if !canContinue {
			break
		}

		p.advance()

		switch {
		case ch == '.':
			// Step 7.4.1: '.' に来た時点で integer 部分は 12 文字以内であること。
			if numBuf.Len() > 12 {
				return nil, p.errAt("integer part too long for decimal")
			}
			numBuf.WriteByte(ch)
			isDecimal = true

		case isDecimal:
			numBuf.WriteByte(ch)
			fracDigits++
			if fracDigits > 3 {
				return nil, p.errAt("decimal fraction too long")
			}

		default:
			numBuf.WriteByte(ch)
			if numBuf.Len() > 15 {
				return nil, p.errAt("integer too long")
			}
		}
	}

	s := numBuf.String()

	// Step 9.1
	if isDecimal && strings.HasSuffix(s, ".") {
		return nil, p.errAt("decimal must have fractional part")
	}

	if isNegative {
		s = "-" + s
	}

	if isDecimal {
		val, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, p.errAt("invalid decimal")
		}

		return Decimal(val), nil
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, p.errAt("invalid integer")
	}

	return Integer(val), nil
}

// parseString - RFC 9651 §4.2.5 String をパースする。
func (p *parser) parseString() (BareItem, error) {
	ch, ok := p.advance()
	if !ok || ch != '"' {
		return nil, p.errAt("expected '\"' at start of string")
	}

	var buf strings.Builder
	buf.Grow(100)

	escaped := false
	for !p.isEmpty() {
		ch, ok := p.peek()
		if !ok {
			return nil, p.errAt("unexpected end of input in string")
		}

		if escaped {
			if ch != '"' && ch != '\\' {
				return nil, p.errAt("invalid escape sequence in string")
			}
			buf.WriteByte(ch)
			escaped = false
			p.advance()
			continue
		}

		switch {
		case ch == '"':
			p.advance()
			return String(buf.String()), nil

		case ch == '\\':
			escaped = true

		case isStringUnescaped(ch):
			buf.WriteByte(ch)

		default:
			return nil, p.errAt("invalid character in string")
		}

		p.advance()
	}

	return nil, p.errAt("unclosed string")
}

// parseToken - RFC 9651 §4.2.6 Token をパースする。
func (p *parser) parseToken() (BareItem, error) {
	ch, ok := p.advance()
	if !ok || (!isAlpha(ch) && ch != '*') {
		return nil, p.errAt("expected ALPHA or '*' at start of token")
	}

	var buf strings.Builder
	buf.Grow(100)

	buf.WriteByte(ch)

	for !p.isEmpty() {
		ch, ok := p.peek()
		if !ok {
			return Token(buf.String()), nil
		}

		if isTchar(ch) || ch == ':' || ch == '/' {
			buf.WriteByte(ch)
			p.advance()
		} else {
			break
		}
	}

	return Token(buf.String()), nil
}

// parseBinarySequence - RFC 9651 §4.2.7 Byte Sequence をパースする。
func (p *parser) parseBinarySequence() (BareItem, error) {
	ch, ok := p.advance()
	if !ok || ch != ':' {
		return nil, p.errAt("expected ':' at start of byte sequence")
	}

	var buf strings.Builder
	buf.Grow(100)

	closed := false
	for !p.isEmpty() {
		ch, ok := p.peek()
		if !ok {
			return nil, p.errAt("unclosed byte sequence")
		}

		if ch == ':' {
			p.advance()
			closed = true
			break
		}

		if !isBase64(ch) {
			return nil, p.errAt("invalid base64 character in byte sequence")
		}

		buf.WriteByte(ch)
		p.advance()
	}

	if !closed {
		return nil, p.errAt("unclosed byte sequence")
	}

	if buf.Len() == 0 {
		return ByteSequence{}, nil
	}

	dec, err := base64.StdEncoding.DecodeString(buf.String())
	if err != nil {
		return nil, p.errAt("invalid base64 in byte sequence")
	}

	return ByteSequence(dec), nil
}

// parseBoolean - RFC 9651 §4.2.8 Boolean をパースする。
func (p *parser) parseBoolean() (BareItem, error) {
	ch, ok := p.advance()
	if !ok || ch != '?' {
		return nil, p.errAt("expected '?' at start of boolean")
	}

	ch, ok = p.advance()
	if !ok {
		return nil, p.errAt("unexpected end of input after '?'")
	}

	switch ch {
	case '0':
		return Boolean(false), nil

	case '1':
		return Boolean(true), nil

	default:
		return nil, p.errAt("boolean must be '0' or '1'")
	}
}

// parseDate - RFC 9651 §4.2.9 Date をパースする。
func (p *parser) parseDate() (BareItem, error) {
	ch, ok := p.advance()
	if !ok || ch != '@' {
		return nil, p.errAt("expected '@' at start of date")
	}

	v, err := p.parseIntegerOrDecimal()
	if err != nil {
		return nil, err
	}

	i, ok := v.(Integer)
	if !ok {
		return nil, p.errAt("date must be an integer")
	}

	return Date(i), nil
}

// parseDisplayString - RFC 9651 §4.2.10 Display String をパースする。
func (p *parser) parseDisplayString() (BareItem, error) {
	ch, ok := p.advance()
	if !ok || ch != '%' {
		return nil, p.errAt("expected '%' at start of display string")
	}

	ch, ok = p.advance()
	if !ok || ch != '"' {
		return nil, p.errAt("expected '\"' after '%' in display string")
	}

	var buf strings.Builder
	buf.Grow(256)

	for !p.isEmpty() {
		ch, ok := p.peek()
		if !ok {
			return nil, p.errAt("unclosed display string")
		}

		switch {
		case ch == '"':
			p.advance()
			str := buf.String()
			if !utf8.ValidString(str) {
				return nil, p.errAt("invalid UTF-8 in display string")
			}
			return DisplayString(str), nil

		case isDisplayStringUnescaped(ch):
			buf.WriteByte(ch)
			p.advance()

		case ch == '%':
			p.advance()
			ch1, ok := p.advance()
			if !ok || !isLowerCaseHexDigit(ch1) {
				return nil, p.errAt("invalid hex digit in display string")
			}

			ch2, ok := p.advance()
			if !ok || !isLowerCaseHexDigit(ch2) {
				return nil, p.errAt("invalid hex digit in display string")
			}

			buf.WriteByte(p.decodePercentEncoded(ch1, ch2))

		default:
			return nil, p.errAt("invalid character in display string")
		}
	}

	return nil, p.errAt("unclosed display string")
}

func (p *parser) decodePercentEncoded(c1, c2 byte) byte {
	c1Val := byte(slices.Index(hexChars, c1))
	c2Val := byte(slices.Index(hexChars, c2))

	return c1Val*16 + c2Val
}

// finalize - RFC 9651 §4.2 step 6-7: 末尾 SP を破棄し、残り入力が無いことを確認する。
func (p *parser) finalize() error {
	p.discardLeadingSP()
	if !p.isEmpty() {
		return p.errAt("trailing content after parse")
	}
	return nil
}
