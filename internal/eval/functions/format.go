package functions

import (
	"fmt"
	"strconv"
	"strings"
)

// Format evaluates a format string with the supplied arguments.
// It behaves like the C# implementation in the repository –
// it supports escaped braces and numeric argument indices.
// Format specifiers (e.g. :D) are recognised but currently ignored.
func Format(formatStr string, args ...interface{}) (string, error) {
	var sb strings.Builder
	i := 0
	for i < len(formatStr) {
		lbrace := strings.IndexByte(formatStr[i:], '{')
		rbrace := strings.IndexByte(formatStr[i:], '}')

		// left brace
		if lbrace >= 0 && (rbrace < 0 || rbrace > lbrace) {
			l := i + lbrace

			sb.WriteString(formatStr[i:l])

			// escaped left brace
			if l+1 < len(formatStr) && formatStr[l+1] == '{' {
				sb.WriteString(formatStr[i : l+2])
				i = l + 2
				continue
			}

			// normal placeholder
			if rbrace > lbrace+1 {
				// read index
				idx, endIdx, ok := readArgIndex(formatStr, l+1)
				if !ok {
					return "", fmt.Errorf("invalid format string: %s", formatStr)
				}
				// read optional format specifier
				spec, r, ok := readFormatSpecifiers(formatStr, endIdx+1)
				if !ok {
					return "", fmt.Errorf("invalid format string: %s", formatStr)
				}
				if idx >= len(args) {
					return "", fmt.Errorf("argument index %d out of range", idx)
				}
				// append argument (format specifier is ignored here)
				arg := args[idx]
				sb.WriteString(fmt.Sprintf("%v", arg))
				if spec != "" {
					// placeholder for future specifier handling
					_ = spec
				}
				i = r + 1
				continue
			}
			return "", fmt.Errorf("invalid format string: %s", formatStr)
		}

		// right brace
		if rbrace >= 0 {
			// escaped right brace
			if rbrace+1 < len(formatStr) && formatStr[rbrace+1] == '}' {
				sb.WriteString(formatStr[i : rbrace+2])
				i = rbrace + 2
				continue
			}
			return "", fmt.Errorf("invalid format string: %s", formatStr)
		}

		// rest of string
		sb.WriteString(formatStr[i:])
		break
	}
	return sb.String(), nil
}

// readArgIndex parses a decimal number starting at pos.
// It returns the parsed value, the index of the last digit and true on success.
func readArgIndex(s string, pos int) (int, int, bool) {
	start := pos
	for pos < len(s) && s[pos] >= '0' && s[pos] <= '9' {
		pos++
	}
	if start == pos {
		return 0, 0, false
	}
	idx, err := strconv.Atoi(s[start:pos])
	if err != nil {
		return 0, 0, false
	}
	return idx, pos - 1, true
}

// readFormatSpecifiers reads an optional format specifier block.
// It returns the specifier string, the index of the closing '}' and true on success.
func readFormatSpecifiers(s string, pos int) (string, int, bool) {
	if pos >= len(s) {
		return "", 0, false
	}
	if s[pos] == '}' {
		return "", pos, true
	}
	if s[pos] != ':' {
		return "", 0, false
	}
	pos++ // skip ':'
	start := pos
	for pos < len(s) {
		if s[pos] == '}' {
			return s[start:pos], pos, true
		}
		if s[pos] == '}' && pos+1 < len(s) && s[pos+1] == '}' {
			// escaped '}'
			pos += 2
			continue
		}
		pos++
	}
	return "", 0, false
}
