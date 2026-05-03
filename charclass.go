package sfv

func isOWS(c byte) bool {
	return c == ' ' || c == '\t'
}

func isSP(c byte) bool {
	return c == ' '
}

func isAlpha(c byte) bool {
	return (c >= 0x41 && c <= 0x5a) || (c >= 0x61 && c <= 0x7a)
}

func isLowerCaseAlpha(c byte) bool {
	return c >= 0x61 && c <= 0x7a
}

func isDigit(c byte) bool {
	return c >= 0x30 && c <= 0x39
}

func isLowerCaseHexDigit(c byte) bool {
	return isDigit(c) || (c >= 0x61 && c <= 0x66)
}

func isTchar(c byte) bool {
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*',
		'+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}
	return isAlpha(c) || isDigit(c)
}

func isBase64(c byte) bool {
	return isAlpha(c) || isDigit(c) || c == '+' || c == '/' || c == '='
}

func isStringUnescaped(c byte) bool {
	return isUnescaped(c) || c == '%'
}

func isDisplayStringUnescaped(c byte) bool {
	return isUnescaped(c) || c == '\\'
}

func isUnescaped(c byte) bool {
	return (c >= 0x20 && c <= 0x21) ||
		(c >= 0x23 && c <= 0x24) ||
		(c >= 0x26 && c <= 0x5b) ||
		(c >= 0x5d && c <= 0x7e)
}

func isKeyStart(c byte) bool {
	return isLowerCaseAlpha(c) || c == '*'
}

func isKeyChar(c byte) bool {
	return isLowerCaseAlpha(c) || isDigit(c) ||
		c == '_' || c == '-' || c == '.' || c == '*'
}
