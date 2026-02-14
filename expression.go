package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ExprNode интерфейс для узла выражения
type ExprNode interface {
	Evaluate(vars map[string]VariableValue) (interface{}, error)
}

// LiteralNode – литерал (число, строка, bool)
type LiteralNode struct {
	Value interface{}
}

func (n *LiteralNode) Evaluate(vars map[string]VariableValue) (interface{}, error) {
	return n.Value, nil
}

// IdentifierNode – переменная
type IdentifierNode struct {
	Name string
}

func (n *IdentifierNode) Evaluate(vars map[string]VariableValue) (interface{}, error) {
	val, ok := vars[n.Name]
	if !ok {
		return nil, fmt.Errorf("неизвестная переменная: %s", n.Name)
	}
	return val.Value, nil
}

// BinaryOpNode – бинарная операция
type BinaryOpNode struct {
	Left  ExprNode
	Op    string // "+", "-", "*", "/", "%", "==", "!=", "<", ">", "<=", ">=", "&&", "||"
	Right ExprNode
}

func (n *BinaryOpNode) Evaluate(vars map[string]VariableValue) (interface{}, error) {
	l, err := n.Left.Evaluate(vars)
	if err != nil {
		return nil, err
	}
	r, err := n.Right.Evaluate(vars)
	if err != nil {
		return nil, err
	}
	// Проверка типов и выполнение операции
	switch n.Op {
	case "+":
		// Числа или строки
		lNum, lOk := toFloat64(l)
		rNum, rOk := toFloat64(r)
		if lOk && rOk {
			return lNum + rNum, nil
		}
		lStr, lOk := l.(string)
		rStr, rOk := r.(string)
		if lOk && rOk {
			return lStr + rStr, nil
		}
		return nil, fmt.Errorf("операция '+' не поддерживается для типов %T и %T", l, r)
	case "-":
		lNum, lOk := toFloat64(l)
		rNum, rOk := toFloat64(r)
		if lOk && rOk {
			return lNum - rNum, nil
		}
		return nil, fmt.Errorf("операция '-' требует числа")
	case "*":
		lNum, lOk := toFloat64(l)
		rNum, rOk := toFloat64(r)
		if lOk && rOk {
			return lNum * rNum, nil
		}
		return nil, fmt.Errorf("операция '*' требует числа")
	case "/":
		lNum, lOk := toFloat64(l)
		rNum, rOk := toFloat64(r)
		if lOk && rOk {
			if rNum == 0 {
				return nil, fmt.Errorf("деление на ноль")
			}
			return lNum / rNum, nil
		}
		return nil, fmt.Errorf("операция '/' требует числа")
	case "%":
		lInt, lOk := toInt64(l)
		rInt, rOk := toInt64(r)
		if lOk && rOk {
			if rInt == 0 {
				return nil, fmt.Errorf("деление на ноль")
			}
			return lInt % rInt, nil
		}
		return nil, fmt.Errorf("операция '%%' требует целых чисел")
	case "==":
		return l == r, nil
	case "!=":
		return l != r, nil
	case "<":
		lNum, lOk := toFloat64(l)
		rNum, rOk := toFloat64(r)
		if lOk && rOk {
			return lNum < rNum, nil
		}
		return nil, fmt.Errorf("операция '<' требует числа")
	case ">":
		lNum, lOk := toFloat64(l)
		rNum, rOk := toFloat64(r)
		if lOk && rOk {
			return lNum > rNum, nil
		}
		return nil, fmt.Errorf("операция '>' требует числа")
	case "<=":
		lNum, lOk := toFloat64(l)
		rNum, rOk := toFloat64(r)
		if lOk && rOk {
			return lNum <= rNum, nil
		}
		return nil, fmt.Errorf("операция '<=' требует числа")
	case ">=":
		lNum, lOk := toFloat64(l)
		rNum, rOk := toFloat64(r)
		if lOk && rOk {
			return lNum >= rNum, nil
		}
		return nil, fmt.Errorf("операция '>=' требует числа")
	case "&&":
		lBool, lOk := l.(bool)
		rBool, rOk := r.(bool)
		if lOk && rOk {
			return lBool && rBool, nil
		}
		return nil, fmt.Errorf("операция '&&' требует bool")
	case "||":
		lBool, lOk := l.(bool)
		rBool, rOk := r.(bool)
		if lOk && rOk {
			return lBool || rBool, nil
		}
		return nil, fmt.Errorf("операция '||' требует bool")
	default:
		return nil, fmt.Errorf("неизвестная операция: %s", n.Op)
	}
}

// UnaryOpNode – унарная операция
type UnaryOpNode struct {
	Op    string // "-", "!"
	Right ExprNode
}

func (n *UnaryOpNode) Evaluate(vars map[string]VariableValue) (interface{}, error) {
	r, err := n.Right.Evaluate(vars)
	if err != nil {
		return nil, err
	}
	switch n.Op {
	case "-":
		num, ok := toFloat64(r)
		if !ok {
			return nil, fmt.Errorf("унарный минус требует число")
		}
		return -num, nil
	case "!":
		b, ok := r.(bool)
		if !ok {
			return nil, fmt.Errorf("унарное '!' требует bool")
		}
		return !b, nil
	default:
		return nil, fmt.Errorf("неизвестная унарная операция: %s", n.Op)
	}
}

// Вспомогательные функции для преобразования
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case float64:
		return val, true
	default:
		return 0, false
	}
}

func toInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int64:
		return val, true
	case float64:
		return int64(val), true
	default:
		return 0, false
	}
}

// Парсер выражений
type parser struct {
	input string
	pos   int
}

func ParseExpression(input string) (ExprNode, error) {
	p := &parser{input: strings.TrimSpace(input)}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.input) {
		return nil, fmt.Errorf("лишние символы в конце: %s", p.input[p.pos:])
	}
	return expr, nil
}

// parseExpr обрабатывает операции низкого приоритета: ||, &&
func (p *parser) parseExpr() (ExprNode, error) {
	node, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.input) {
			break
		}
		if strings.HasPrefix(p.input[p.pos:], "||") {
			p.pos += 2
			right, err := p.parseAndExpr()
			if err != nil {
				return nil, err
			}
			node = &BinaryOpNode{Left: node, Op: "||", Right: right}
		} else {
			break
		}
	}
	return node, nil
}

// parseAndExpr обрабатывает &&
func (p *parser) parseAndExpr() (ExprNode, error) {
	node, err := p.parseEqualityExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpaces()
		if strings.HasPrefix(p.input[p.pos:], "&&") {
			p.pos += 2
			right, err := p.parseEqualityExpr()
			if err != nil {
				return nil, err
			}
			node = &BinaryOpNode{Left: node, Op: "&&", Right: right}
		} else {
			break
		}
	}
	return node, nil
}

// parseEqualityExpr обрабатывает ==, !=
func (p *parser) parseEqualityExpr() (ExprNode, error) {
	node, err := p.parseRelationalExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpaces()
		if strings.HasPrefix(p.input[p.pos:], "==") {
			p.pos += 2
			right, err := p.parseRelationalExpr()
			if err != nil {
				return nil, err
			}
			node = &BinaryOpNode{Left: node, Op: "==", Right: right}
		} else if strings.HasPrefix(p.input[p.pos:], "!=") {
			p.pos += 2
			right, err := p.parseRelationalExpr()
			if err != nil {
				return nil, err
			}
			node = &BinaryOpNode{Left: node, Op: "!=", Right: right}
		} else {
			break
		}
	}
	return node, nil
}

// parseRelationalExpr обрабатывает <, >, <=, >=
func (p *parser) parseRelationalExpr() (ExprNode, error) {
	node, err := p.parseAddExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpaces()
		if strings.HasPrefix(p.input[p.pos:], "<=") {
			p.pos += 2
			right, err := p.parseAddExpr()
			if err != nil {
				return nil, err
			}
			node = &BinaryOpNode{Left: node, Op: "<=", Right: right}
		} else if strings.HasPrefix(p.input[p.pos:], ">=") {
			p.pos += 2
			right, err := p.parseAddExpr()
			if err != nil {
				return nil, err
			}
			node = &BinaryOpNode{Left: node, Op: ">=", Right: right}
		} else if p.pos < len(p.input) && p.input[p.pos] == '<' {
			p.pos++
			right, err := p.parseAddExpr()
			if err != nil {
				return nil, err
			}
			node = &BinaryOpNode{Left: node, Op: "<", Right: right}
		} else if p.pos < len(p.input) && p.input[p.pos] == '>' {
			p.pos++
			right, err := p.parseAddExpr()
			if err != nil {
				return nil, err
			}
			node = &BinaryOpNode{Left: node, Op: ">", Right: right}
		} else {
			break
		}
	}
	return node, nil
}

// parseAddExpr обрабатывает +, -
func (p *parser) parseAddExpr() (ExprNode, error) {
	node, err := p.parseMulExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.input) {
			break
		}
		ch := p.input[p.pos]
		if ch == '+' || ch == '-' {
			p.pos++
			right, err := p.parseMulExpr()
			if err != nil {
				return nil, err
			}
			node = &BinaryOpNode{Left: node, Op: string(ch), Right: right}
		} else {
			break
		}
	}
	return node, nil
}

// parseMulExpr обрабатывает *, /, %
func (p *parser) parseMulExpr() (ExprNode, error) {
	node, err := p.parseUnaryExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.input) {
			break
		}
		ch := p.input[p.pos]
		if ch == '*' || ch == '/' || ch == '%' {
			p.pos++
			right, err := p.parseUnaryExpr()
			if err != nil {
				return nil, err
			}
			node = &BinaryOpNode{Left: node, Op: string(ch), Right: right}
		} else {
			break
		}
	}
	return node, nil
}

// parseUnaryExpr обрабатывает унарные +, -, !, а также скобки и литералы
func (p *parser) parseUnaryExpr() (ExprNode, error) {
	p.skipSpaces()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("неожиданный конец выражения")
	}
	ch := p.input[p.pos]
	if ch == '+' || ch == '-' || ch == '!' {
		p.pos++
		right, err := p.parseUnaryExpr()
		if err != nil {
			return nil, err
		}
		return &UnaryOpNode{Op: string(ch), Right: right}, nil
	}
	return p.parsePrimary()
}

// parsePrimary обрабатывает числа, строки, true/false, идентификаторы, скобки
func (p *parser) parsePrimary() (ExprNode, error) {
	p.skipSpaces()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("ожидается выражение")
	}
	ch := p.input[p.pos]
	if ch == '(' {
		p.pos++
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		p.skipSpaces()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return nil, fmt.Errorf("ожидается ')'")
		}
		p.pos++
		return expr, nil
	}
	if ch == '"' {
		// строковый литерал
		p.pos++
		start := p.pos
		for p.pos < len(p.input) && p.input[p.pos] != '"' {
			p.pos++
		}
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("незакрытая строка")
		}
		str := p.input[start:p.pos]
		p.pos++
		return &LiteralNode{Value: str}, nil
	}
	if unicode.IsDigit(rune(ch)) || ch == '.' {
		// число
		start := p.pos
		for p.pos < len(p.input) && (unicode.IsDigit(rune(p.input[p.pos])) || p.input[p.pos] == '.') {
			p.pos++
		}
		numStr := p.input[start:p.pos]
		if strings.Contains(numStr, ".") {
			val, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return nil, err
			}
			return &LiteralNode{Value: val}, nil
		} else {
			val, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return nil, err
			}
			return &LiteralNode{Value: int(val)}, nil // храним как int
		}
	}
	// идентификатор или true/false
	start := p.pos
	for p.pos < len(p.input) && (unicode.IsLetter(rune(p.input[p.pos])) || unicode.IsDigit(rune(p.input[p.pos])) || p.input[p.pos] == '_') {
		p.pos++
	}
	ident := p.input[start:p.pos]

	// Проверяем, не является ли идентификатор названием цвета
	if idx, ok := colorNameToIndex(ident); ok {
		return &LiteralNode{Value: int(idx)}, nil
	}

	switch ident {
	case "true":
		return &LiteralNode{Value: true}, nil
	case "false":
		return &LiteralNode{Value: false}, nil
	default:
		return &IdentifierNode{Name: ident}, nil
	}
}

func (p *parser) skipSpaces() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func colorNameToIndex(name string) (byte, bool) {
	name = strings.ToLower(name)
	idx, ok := NameToIndexColor[name]
	return idx, ok
}
