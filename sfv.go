package sfv

import (
	"strings"
)

var hexChars = []byte("0123456789abcdef")

func joinFieldLines(raw []string) string {
	if len(raw) == 1 {
		return raw[0]
	}
	return strings.Join(raw, ", ")
}

func newParser(input string) *parser {
	p := &parser{input: input}
	p.discardLeadingSP()
	return p
}

// ParseList - RFC 9651 §4.2.1 List をパースする。
func ParseList(raw []string) (List, error) {
	p := newParser(joinFieldLines(raw))
	result, err := p.parseList()
	if err != nil {
		return nil, err
	}
	if err := p.finalize(); err != nil {
		return nil, err
	}
	return result, nil
}

// ParseDictionary - RFC 9651 §4.2.2 Dictionary をパースする。
func ParseDictionary(raw []string) (*Dictionary, error) {
	p := newParser(joinFieldLines(raw))
	result, err := p.parseDictionary()
	if err != nil {
		return nil, err
	}
	if err := p.finalize(); err != nil {
		return nil, err
	}
	return result, nil
}

// ParseItem - RFC 9651 §4.2.3 Item をパースする。
func ParseItem(raw []string) (*Item, error) {
	p := newParser(joinFieldLines(raw))
	result, err := p.parseItem()
	if err != nil {
		return nil, err
	}
	if err := p.finalize(); err != nil {
		return nil, err
	}
	return result, nil
}
