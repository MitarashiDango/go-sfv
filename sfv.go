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

// ParseList - RFC 9651 Section 4.2.1 List をパースする。
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

// ParseDictionary - RFC 9651 Section 4.2.2 Dictionary をパースする。
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

// ParseItem - RFC 9651 Section 4.2.3 Item をパースする。
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

// SerializeList - RFC 9651 Section 4.1.1 List をシリアライズする。
func SerializeList(l List) (string, error) {
	s := &serializer{}
	if err := s.writeList(l); err != nil {
		return "", err
	}
	return s.String(), nil
}

// SerializeDictionary - RFC 9651 Section 4.1.2 Dictionary をシリアライズする。
func SerializeDictionary(d *Dictionary) (string, error) {
	s := &serializer{}
	if err := s.writeDictionary(d); err != nil {
		return "", err
	}
	return s.String(), nil
}

// SerializeItem - RFC 9651 Section 4.1.3 Item をシリアライズする。
func SerializeItem(i *Item) (string, error) {
	s := &serializer{}
	if err := s.writeItem(i); err != nil {
		return "", err
	}
	return s.String(), nil
}
