package sfv

import "fmt"

// ParseError - パース失敗時のエラー。発生位置 Pos を保持する。
type ParseError struct {
	Pos     int
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("sfv: %s (pos %d)", e.Message, e.Pos)
}

func (p *parser) errAt(msg string) error {
	return &ParseError{Pos: p.pos, Message: msg}
}
