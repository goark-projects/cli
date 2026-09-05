package condition

import (
	"fmt"
	"strings"

	"goark.dev/cli/internal/envutil"
)

// Values 是 when 表达式允许访问的只读值。
type Values struct {
	Profile     string
	GOOS        string
	GOARCH      string
	Environment map[string]string
}

// Evaluate 解析并计算无副作用的 when 表达式。
func Evaluate(expression string, values Values) (bool, error) {
	if strings.TrimSpace(expression) == "" {
		return true, nil
	}
	tokens, err := lex(expression)
	if err != nil {
		return false, err
	}
	parser := expressionParser{tokens: tokens, values: values}
	result, err := parser.parseOr()
	if err != nil {
		return false, err
	}
	if current := parser.current(); current.kind != tokenEOF {
		return false, syntaxError(current.position, fmt.Sprintf("存在多余内容 %q", current.text))
	}
	if result.kind != valueBoolean {
		return false, syntaxError(tokens[0].position, "表达式结果必须是布尔值")
	}
	return result.boolean, nil
}

type valueKind uint8

const (
	valueString valueKind = iota
	valueBoolean
)

type expressionValue struct {
	kind    valueKind
	text    string
	boolean bool
}

type expressionParser struct {
	tokens []token
	index  int
	values Values
}

func (p *expressionParser) parseOr() (expressionValue, error) {
	left, err := p.parseAnd()
	if err != nil {
		return expressionValue{}, err
	}
	for p.current().kind == tokenOr {
		operator := p.consume()
		right, err := p.parseAnd()
		if err != nil {
			return expressionValue{}, err
		}
		left, err = logical(operator, left, right, false)
		if err != nil {
			return expressionValue{}, err
		}
	}
	return left, nil
}

func (p *expressionParser) parseAnd() (expressionValue, error) {
	left, err := p.parseComparison()
	if err != nil {
		return expressionValue{}, err
	}
	for p.current().kind == tokenAnd {
		operator := p.consume()
		right, err := p.parseComparison()
		if err != nil {
			return expressionValue{}, err
		}
		left, err = logical(operator, left, right, true)
		if err != nil {
			return expressionValue{}, err
		}
	}
	return left, nil
}

func (p *expressionParser) parseComparison() (expressionValue, error) {
	left, err := p.parseUnary()
	if err != nil {
		return expressionValue{}, err
	}
	operator := p.current()
	if operator.kind != tokenEqual && operator.kind != tokenNotEqual {
		return left, nil
	}
	p.consume()
	right, err := p.parseUnary()
	if err != nil {
		return expressionValue{}, err
	}
	if left.kind != right.kind {
		return expressionValue{}, syntaxError(operator.position, "比较两侧类型必须一致")
	}
	equal := left.text == right.text && left.boolean == right.boolean
	if operator.kind == tokenNotEqual {
		equal = !equal
	}
	return expressionValue{kind: valueBoolean, boolean: equal}, nil
}

func (p *expressionParser) parseUnary() (expressionValue, error) {
	current := p.current()
	if current.kind == tokenNot {
		p.consume()
		value, err := p.parseUnary()
		if err != nil {
			return expressionValue{}, err
		}
		if value.kind != valueBoolean {
			return expressionValue{}, syntaxError(current.position, "逻辑非只接受布尔值")
		}
		value.boolean = !value.boolean
		return value, nil
	}
	return p.parsePrimary()
}

func (p *expressionParser) parsePrimary() (expressionValue, error) {
	current := p.consume()
	switch current.kind {
	case tokenString:
		return expressionValue{kind: valueString, text: current.text}, nil
	case tokenTrue:
		return expressionValue{kind: valueBoolean, boolean: true}, nil
	case tokenFalse:
		return expressionValue{kind: valueBoolean, boolean: false}, nil
	case tokenIdentifier:
		value, err := p.resolveIdentifier(current)
		return expressionValue{kind: valueString, text: value}, err
	case tokenLeftParen:
		value, err := p.parseOr()
		if err != nil {
			return expressionValue{}, err
		}
		if p.current().kind != tokenRightParen {
			return expressionValue{}, syntaxError(p.current().position, "缺少右括号")
		}
		p.consume()
		return value, nil
	default:
		return expressionValue{}, syntaxError(current.position, fmt.Sprintf("意外内容 %q", current.text))
	}
}

func (p *expressionParser) resolveIdentifier(current token) (string, error) {
	switch current.text {
	case "profile":
		return p.values.Profile, nil
	case "goos":
		return p.values.GOOS, nil
	case "goarch":
		return p.values.GOARCH, nil
	}
	if name, ok := strings.CutPrefix(current.text, "env."); ok && name != "" {
		value, exists := envutil.Lookup(p.values.Environment, name)
		if !exists {
			return "", syntaxError(current.position, fmt.Sprintf("环境变量 %q 未定义", name))
		}
		return value, nil
	}
	return "", syntaxError(current.position, fmt.Sprintf("未知变量 %q", current.text))
}

func logical(operator token, left expressionValue, right expressionValue, and bool) (expressionValue, error) {
	if left.kind != valueBoolean || right.kind != valueBoolean {
		return expressionValue{}, syntaxError(operator.position, "逻辑操作符只接受布尔值")
	}
	if and {
		return expressionValue{kind: valueBoolean, boolean: left.boolean && right.boolean}, nil
	}
	return expressionValue{kind: valueBoolean, boolean: left.boolean || right.boolean}, nil
}

func (p *expressionParser) current() token {
	return p.tokens[p.index]
}

func (p *expressionParser) consume() token {
	current := p.current()
	if p.index < len(p.tokens)-1 {
		p.index++
	}
	return current
}
