package whtml

import (
	"bufio"
	"io"
)

type NodeType uint32

const (
	ErrorNode NodeType = iota
	TextNode
	ElementNode
	MustacheNode
)

type Node struct {
	Type                     NodeType
	Data                     string
	Attrs                    []Attribute
	Parent                   *Node
	FirstChild, LastChild    *Node
	PrevSibling, NextSibling *Node

	Namespace string
	Pos       Pos
}

func (node *Node) Render(writer io.Writer) {
	buf := bufio.NewWriter(writer)
	node.render(buf, 0)
	buf.Flush()
}

func (node *Node) render(w *bufio.Writer, depth int) {
	for i := 0; i < depth; i++ {
		w.WriteString("  ")
	}

	switch node.Type {
	case ElementNode:
		w.WriteRune('<')
		w.WriteString(node.Data)
		if len(node.Attrs) > 0 {
			for _, attr := range node.Attrs {
				w.WriteRune(' ')
				switch attr.Type {
				case StringAttribute:
					w.WriteString(sfmt("%v=\"%v\"", attr.Key, attr.Val))
				case MustacheAttribute:
					w.WriteString(sfmt("%v={{%v}}", attr.Key, attr.Val))
				case BoolAttribute:
					w.WriteString(attr.Key)
				case VariadicAttribute:
					w.WriteString(sfmt("...{{%v}}", attr.Val))
				}
			}
		}

		w.WriteString(">\n")

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			c.render(w, depth+1)
		}

		w.WriteRune('\n')

		for i := 0; i < depth; i++ {
			w.WriteString("  ")
		}

		w.WriteString(sfmt("</%v>", node.Data))

	case TextNode:
		w.WriteString(node.Data)
	}

	w.WriteRune('\n')
}

// AppendChild adds a node c as a child of n.
//
// It will panic if c already has a parent or siblings.
func (n *Node) AppendChild(c *Node) {
	if c.Parent != nil || c.PrevSibling != nil || c.NextSibling != nil {
		panic("html: AppendChild called for an attached child Node")
	}
	last := n.LastChild
	if last != nil {
		last.NextSibling = c
	} else {
		n.FirstChild = c
	}
	n.LastChild = c
	c.Parent = n
	c.PrevSibling = last
}

// nodeStack is a stack of nodes.
type nodeStack []*Node

func (s *nodeStack) push(node *Node) {
	*s = append(*s, node)
}

// pop pops the stack. It will panic if s is empty.
func (s *nodeStack) pop() *Node {
	i := len(*s)
	n := (*s)[i-1]
	*s = (*s)[:i-1]
	return n
}

// top returns the most recently pushed node, or nil if s is empty.
func (s *nodeStack) top() *Node {
	if i := len(*s); i > 0 {
		return (*s)[i-1]
	}
	return nil
}
