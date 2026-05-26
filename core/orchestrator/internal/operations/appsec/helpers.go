package appsec

import "strings"

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func itoaPositive(n int) string {
	if n < 0 {
		return "-" + itoaPositive(-n)
	}
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func lineForOffset(content string, offset int) int {
	if offset <= 0 {
		return 1
	}
	line := 1
	for i, r := range content {
		if i >= offset {
			break
		}
		if r == '\n' {
			line++
		}
	}
	return line
}
