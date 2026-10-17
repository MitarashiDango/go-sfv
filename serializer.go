package sfv

import (
	"strings"
)

type serializer struct {
	buf strings.Builder
}

func (s *serializer) writeList(list List) error {
	if len(list) == 0 {
		return nil
	}

	for i, listMember := range list {
		if i > 0 {
			s.buf.WriteString(", ")
		}

		if innerList, ok := listMember.(*InnerList); ok {
			s.writeInnerList(innerList)
		} else if item, ok := listMember.(*Item); ok {
			s.writeItem(item)
		}
	}

	return newSerializeError("not implemented")
}

func (s *serializer) writeDictionary(d *Dictionary) error {
	return newSerializeError("not implemented")
}

func (s *serializer) writeItem(i *Item) error {
	return newSerializeError("not implemented")
}

func (s *serializer) writeBareItem(b BareItem) error {
	return newSerializeError("not implemented")
}

func (s *serializer) writeInnerList(d *InnerList) error {
	return newSerializeError("not implemented")
}

func (s *serializer) String() string {
	return s.buf.String()
}

// isValidInteger - RFC 9651 Section 3.3.1 Integer の値が有効な範囲内かどうかをチェックする。
func isValidInteger(i Integer) bool {
	return i >= minIntegerValue && i <= maxIntegerValue
}
