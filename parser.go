package whtml

import (
	"fmt"
	"io"
)

// Error represenns a syntax error.
type Error struct {
	Message string
	Pos     Pos
}

// Error returns the formatted string error message.
func (e *Error) Error() string {
	return sfmt("%v:%v: %v", e.Pos.Line+1, e.Pos.Char, e.Message)
}

// ErrorList represenns a list of syntax errors.
type ErrorList []error

// Error returns the formatted string error message.
func (a ErrorList) Error() string {
	switch len(a) {
	case 0:
		return "no errors"
	case 1:
		return a[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", a[0], len(a)-1)
}

type Parser struct {
	s    *Scanner
	ns   *nodeStack
	root *Node
}

func (z *Parser) newNode(tok *Token) *Node {
	switch tok.Type {
	case StartTagToken, SelfClosingTagToken:
		return &Node{
			Type:  ElementNode,
			Data:  tok.Data,
			Attrs: tok.Attrs,
			Pos:   tok.Pos,
		}

	case TextToken:
		return &Node{
			Type: TextNode,
			Data: tok.Data,
			Pos:  tok.Pos,
		}

	case MustacheToken:
		return &Node{
			Type: MustacheNode,
			Data: tok.Data,
			Pos:  tok.Pos,
		}

	default:
		panic("This token type cannot be used for node creation")
	}

	return nil
}

func (z *Parser) addChildNode(node *Node) {
	parent := z.ns.top()
	if parent == nil {
		parent = z.root
	}

	parent.AppendChild(node)
}

func (z *Parser) parse() error {
	for {
		tok := z.s.Scan()
		switch tok.Type {
		case ErrorToken:
			return z.s.Errors[0]

		case EOFToken:
			opening := z.ns.top()
			if opening != nil {
				return &Error{
					Message: sfmt("Unexpected end-of-file. "+
						"Expecting closing tag for %v", opening.Data),
					Pos: tok.Pos,
				}
			}

			return nil

		case StartTagToken:
			node := z.newNode(tok)
			z.addChildNode(node)
			z.ns.push(node)

		case SelfClosingTagToken, TextToken, MustacheToken:
			node := z.newNode(tok)
			z.addChildNode(node)

		case EndTagToken:
			opening := z.ns.top()
			if opening == nil || opening.Data != tok.Data {
				return &Error{
					Message: sfmt("Closing tag '%v' not matching.", tok.Data),
					Pos:     tok.Pos,
				}
			}

			z.ns.pop()
		}
	}

	return nil
}

func Parse(rd io.Reader) ([]*Node, error) {
	s := NewScanner(rd)
	p := &Parser{
		ns:   &nodeStack{},
		s:    s,
		root: &Node{},
	}

	err := p.parse()
	if err != nil {
		return nil, err
	}

	var l []*Node
	for c := p.root.FirstChild; c != nil; c = c.NextSibling {
		l = append(l, c)
	}

	return l, nil
}
