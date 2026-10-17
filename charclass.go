package sfv

// 文字クラス判定。RFC 9651 のパース・シリアライズはいずれも ASCII 文字列をバイト単位で処理するため、
// 引数は byte で受け取る (rune で受け取ると range による UTF-8 デコード後の値を渡す誤用を許してしまう)。

// isOWS - RFC 9110 Section 5.6.3 OWS を構成する文字 (SP / HTAB) かどうかを返す。
func isOWS(c byte) bool {
	return c == ' ' || c == '\t'
}

// isSP - RFC 5234 Appendix B.1 SP (%x20) かどうかを返す。
func isSP(c byte) bool {
	return c == ' '
}

// isAlpha - RFC 5234 Appendix B.1 ALPHA (%x41-5A / %x61-7A) かどうかを返す。
func isAlpha(c byte) bool {
	return (c >= 0x41 && c <= 0x5a) || (c >= 0x61 && c <= 0x7a)
}

// isLowerCaseAlpha - RFC 9651 Appendix C lcalpha (%x61-7A) かどうかを返す。
func isLowerCaseAlpha(c byte) bool {
	return c >= 0x61 && c <= 0x7a
}

// isDigit - RFC 5234 Appendix B.1 DIGIT (%x30-39) かどうかを返す。
func isDigit(c byte) bool {
	return c >= 0x30 && c <= 0x39
}

// isLowerCaseHexDigit - RFC 9651 Appendix C lc-hexdig (DIGIT / %x61-66) かどうかを返す。
// Display String の percent-encoding では大文字の 16 進数字を受け付けない (Section 4.2.10)。
func isLowerCaseHexDigit(c byte) bool {
	return isDigit(c) || (c >= 0x61 && c <= 0x66)
}

// isTchar - RFC 9110 Section 5.6.2 tchar かどうかを返す。
// Token は tchar に加えて ":" と "/" を許すが、それは呼び出し側で判定する (RFC 9651 Section 3.3.4)。
func isTchar(c byte) bool {
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*',
		'+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}
	return isAlpha(c) || isDigit(c)
}

// isBase64 - RFC 9651 Section 4.2.7 step 6 で Byte Sequence に許される文字 (ALPHA / DIGIT / "+" / "/" / "=") かどうかを返す。
// "=" の位置の妥当性は判定しない (base64 デコード時に検出する)。
func isBase64(c byte) bool {
	return isAlpha(c) || isDigit(c) || c == '+' || c == '/' || c == '='
}

// isStringUnescaped - String 内でエスケープせずに現れてよい文字 (unescaped / "%") かどうかを返す。
// RFC 9651 Appendix C sf-string 及び Section 4.2.5 に対応し、SP / VCHAR から DQUOTE と "\" を除いた範囲になる。
func isStringUnescaped(c byte) bool {
	return isUnescaped(c) || c == '%'
}

// isDisplayStringUnescaped - Display String 内でそのまま現れてよい文字 (unescaped / "\") かどうかを返す。
// RFC 9651 Appendix C sf-displaystring 及び Section 4.2.10 に対応し、SP / VCHAR から DQUOTE と "%" を除いた範囲になる。
func isDisplayStringUnescaped(c byte) bool {
	return isUnescaped(c) || c == '\\'
}

// isUnescaped - RFC 9651 Appendix C unescaped (%x20-21 / %x23-24 / %x26-5B / %x5D-7E) かどうかを返す。
// SP / VCHAR から DQUOTE、"%"、"\" を除いた範囲。
func isUnescaped(c byte) bool {
	return (c >= 0x20 && c <= 0x21) ||
		(c >= 0x23 && c <= 0x24) ||
		(c >= 0x26 && c <= 0x5b) ||
		(c >= 0x5d && c <= 0x7e)
}

// isKeyStart - RFC 9651 Appendix C key の先頭に許される文字 (lcalpha / "*") かどうかを返す。
func isKeyStart(c byte) bool {
	return isLowerCaseAlpha(c) || c == '*'
}

// isKeyChar - RFC 9651 Appendix C key の 2 文字目以降に許される文字 (lcalpha / DIGIT / "_" / "-" / "." / "*") かどうかを返す。
func isKeyChar(c byte) bool {
	return isLowerCaseAlpha(c) || isDigit(c) ||
		c == '_' || c == '-' || c == '.' || c == '*'
}
