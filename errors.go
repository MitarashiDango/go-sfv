package sfv

import (
	"fmt"
)

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

// SerializeError - シリアライズ失敗時のエラー。Err には失敗の分類 (ErrValueOutOfRange など) を保持する。
type SerializeError struct {
	Message string
	Err     error
}

func (e *SerializeError) Error() string {
	return fmt.Sprintf("sfv: %s", e.Message)
}

// Unwrap - errors.Is / errors.As のために失敗の分類 Err を返す。
func (e *SerializeError) Unwrap() error {
	return e.Err
}

func newSerializeError(msg string) error {
	return &SerializeError{Message: msg}
}

// wrapSerializeError - 失敗の分類 err を保持する SerializeError を生成する。
func wrapSerializeError(err error, msg string) error {
	return &SerializeError{Message: msg, Err: err}
}
