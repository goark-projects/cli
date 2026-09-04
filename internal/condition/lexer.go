package condition

import (
	"fmt"
	"strconv"
)

type tokenKind uint8

const (
	tokenEOF tokenKind = iota
	tokenIdentifier
	tokenString
	tokenTrue
	tokenFalse
	tokenEqual
	tokenNotEqual
	tokenAnd
	tokenOr
	tokenNot
	tokenLeftParen
	tokenRightParen
)

type token struct {
	kind     tokenKind
	text     string
	position int
}

func lex(input string) ([]token, error) {
	tokens := make([]token, 0, 8)
	for index := 0; index < len(input); {
		if isSpace(input[index]) {
			index++
			continue
		}
		position := index + 1
		switch input[index] {
		case '(':
			tokens = append(tokens, token{kind: tokenLeftParen, text: "(", position: position})
			index++
		case ')':
			tokens = append(tokens, token{kind: tokenRightParen, text: ")", position: position})
			index++
		case '!':
			if index+1 < len(input) && input[index+1] == '=' {
				tokens = append(tokens, token{kind: tokenNotEqual, text: "!=", position: position})
				index += 2
			} else {
				tokens = append(tokens, token{kind: tokenNot, text: "!", position: position})
				index++
			}
		case '=':
			if index+1 >= len(input) || input[index+1] != '=' {
				return nil, syntaxError(position, "只允许 == 比较")
			}
			tokens = append(tokens, token{kind: tokenEqual, text: "==", position: position})
			index += 2
		case '&':
			if index+1 >= len(input) || input[index+1] != '&' {
				return nil, syntaxError(position, "只允许 && 逻辑与")
			}
			tokens = append(tokens, token{kind: tokenAnd, text: "&&", position: position})
			index += 2
		case '|':
			if index+1 >= len(input) || input[index+1] != '|' {
				return nil, syntaxError(position, "只允许 || 逻辑或")
			}
			tokens = append(tokens, token{kind: tokenOr, text: "||", position: position})
			index += 2
		case '"':
			end, value, err := scanString(input, index)
			if err != nil {
				return nil, syntaxError(position, err.Error())
			}
			tokens = append(tokens, token{kind: tokenString, text: value, position: position})
			index = end
		default:
			if !isIdentifierStart(input[index]) {
				return nil, syntaxError(position, fmt.Sprintf("非法字符 %q", input[index]))
			}
			end := index + 1
			for end < len(input) && isIdentifierPart(input[end]) {
				end++
			}
			text := input[index:end]
			kind := tokenIdentifier
			if text == "true" {
				kind = tokenTrue
			} else if text == "false" {
				kind = tokenFalse
			}
			tokens = append(tokens, token{kind: kind, text: text, position: position})
			index = end
		}
	}
	tokens = append(tokens, token{kind: tokenEOF, position: len(input) + 1})
	return tokens, nil
}

func scanString(input string, start int) (int, string, error) {
	escaped := false
	for index := start + 1; index < len(input); index++ {
		if escaped {
			escaped = false
			continue
		}
		if input[index] == '\\' {
			escaped = true
			continue
		}
		if input[index] == '"' {
			raw := input[start : index+1]
			value, err := strconv.Unquote(raw)
			return index + 1, value, err
		}
	}
	return 0, "", fmt.Errorf("字符串缺少结束引号")
}

func isSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n'
}

func isIdentifierStart(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value == '_'
}

func isIdentifierPart(value byte) bool {
	return isIdentifierStart(value) || value >= '0' && value <= '9' || value == '.'
}

func syntaxError(position int, message string) error {
	return fmt.Errorf("when 表达式位置 %d: %s", position, message)
}
