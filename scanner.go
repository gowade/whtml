package whtml

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	//"strings"
)

var (
	rawTags = map[string]bool{
		"iframe":   true,
		"noembed":  true,
		"noframes": true,
		"noscript": true,
		"script":   true,
		"style":    true,
		"textarea": true,
		"title":    true,
		"xmp":      true,
	}
)

func sfmt(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// The Char and Line are both zero-based indexes.
type Pos struct {
	Char int
	Line int
}

// eof represents an EOF file byte.
var eof rune = -1

type Scanner struct {
	// Errors contains a list of all errors that occur during scanning.
	Errors ErrorList

	rd io.RuneReader

	tokbuf      *Token // last token read from the scanner.
	tokbufn     bool   // whether the token buffer is in use.
	nextTokType TokenType

	buf    [4]rune // circular buffer for runes
	bufpos [4]Pos  // circular buffer for position
	bufi   int     // circular buffer index
	bufn   int     // number of buffered characters

	inRawTag  string
	nextToken *Token
}

// New returns a new instance of Scanner.
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{rd: bufio.NewReader(r)}
}

func (s *Scanner) setError(err *Error) *Token {
	s.Errors = append(s.Errors, err)
	return &Token{Type: ErrorToken}
}

// Scan returns the next token from the reader.
func (s *Scanner) Scan() *Token {
	// If unscan was the last call then return the previous token again.
	if s.tokbufn {
		s.tokbufn = false
		return s.tokbuf
	}

	// Otherwise read from the reader and save the token.
	tok := s.scan()
	s.tokbuf = tok
	return tok
}

func newError(message string, pos Pos) *Error {
	return &Error{Message: message, Pos: pos}
}

func unexpected(ch rune, pos Pos) *Error {
	s := sfmt("%q", ch)
	switch ch {
	case eof:
		s = "end-of-file"
	case '\n':
		s = "end-of-line"
	}
	return newError(sfmt("unexpected %v", s), pos)
}

func (s *Scanner) scanMustache() (string, *Error) {
	pos := s.pos()
	var buf bytes.Buffer

	for {
		ch := s.read()
		switch ch {
		case eof, '\n':
			return buf.String(), unexpected(ch, pos)

		case '}':
			next := s.read()
			pos := s.pos()
			if next == '}' {
				if buf.Len() == 0 {
					return "", &Error{Message: "empty mustache", Pos: pos}
				}
				return buf.String(), nil
			} else {
				s.unread(1)
			}
			fallthrough

		default:
			buf.WriteRune(ch)
		}
	}
}

func (s *Scanner) scanString(quo rune) (str string, mustaches []string, err *Error) {
	pos := s.pos()
	var buf bytes.Buffer

	for {
		ch := s.read()
		switch ch {
		case eof, '\n':
			return buf.String(), nil, unexpected(ch, pos)

		case quo:
			return buf.String(), mustaches, nil

		case '{':
			next := s.read()
			if next == '{' {
				m, err := s.scanMustache()
				if err != nil {
					return buf.String(), mustaches, err
				}

				buf.WriteString("%v")
				mustaches = append(mustaches, m)

			} else {
				s.unread(1)
			}
			fallthrough

		default:
			buf.WriteRune(ch)
		}
	}
}

func (s *Scanner) scanAttr() (attr Attribute, err *Error) {
	attr.Pos = s.pos()
	attr.Key = s.scanName()
	attr.Type = BoolAttribute
	for {
		ch := s.read()

		switch ch {
		case '=':
			ch := s.read()
			switch ch {
			case '{':
				next := s.read()
				if next == '{' {
					attr.Type = MustacheAttribute
					attr.Val, err = s.scanMustache()
					if err != nil {
						return
					}
				} else {
					s.unread(1)
				}

			case '\'', '"':
				attr.Type = StringAttribute
				attr.Val, attr.Mustaches, err = s.scanString(ch)
				if err != nil {
					return
				}

			default:
				return attr, unexpected(ch, s.pos())
			}
		default:
			s.unread(1)
			return
		}
	}
}

func (s *Scanner) scanAttrs() (attrs []Attribute, err *Error) {
	for {
		s.skipWhitespace()
		ch := s.read()

		switch {
		case ch == eof:
			return

		case ch == '.':
			// check for variadic attribute
			pos := s.pos()
			nx1 := s.read()
			nx2 := s.read()
			if nx1 == '.' && nx2 == '.' {
				attr := Attribute{
					Type: VariadicAttribute,
					Pos:  pos,
				}

				next := s.read()
				if next == '{' {
					if s.read() == '{' {
						attr.Val, err = s.scanMustache()
						if err != nil {
							return
						}
					} else {
						return attrs, unexpected(next, s.pos())
					}
				} else {
					s.unread(1)
				}

				attrs = append(attrs, attr)
			} else {
				s.unread(2)
				return attrs, unexpected(ch, s.pos())
			}

		case isNameStart(ch):
			s.unread(1)
			attr, err := s.scanAttr()
			if err != nil {
				return attrs, err
			} else {
				attr.Val = string(unescape([]byte(attr.Val), true))
				attrs = append(attrs, attr)
			}

		case ch == '>' || ch == '/':
			s.unread(1)
			return

		default:
			return attrs, unexpected(ch, s.pos())
		}
	}
}

func (s *Scanner) scanStartTag() *Token {
	startPos := s.pos()
	tagName := s.scanName()
	ret := &Token{
		Type: StartTagToken,
		Data: tagName,
		Pos:  startPos,
	}

	for {
		ch := s.read()
		pos := s.pos()

		switch {
		case ch == eof:
			return s.setError(unexpected(eof, pos))

		case ch == '/':
			next := s.read()
			switch next {
			case '>':
				ret.Type = SelfClosingTagToken
				return ret
			default:
				s.unread(1)
				return s.setError(&Error{
					Message: sfmt("%q expected, got %q instead", '>', next),
					Pos:     pos,
				})
			}

		case ch == '>':
			if voidElements[ret.Data] {
				ret.Type = SelfClosingTagToken
			}
			return ret

		case isWhitespace(ch):
			var err *Error
			ret.Attrs, err = s.scanAttrs()
			if err != nil {
				return s.setError(err)
			}

		default:
			return s.setError(unexpected(ch, pos))
		}
	}
}

func (s *Scanner) scanEndTag() *Token {
	pos := s.pos()
	tagName := s.scanName()
	if ch := s.read(); ch != '>' {
		return s.setError(unexpected(ch, s.pos()))
	}

	if voidElements[tagName] {
		return s.setError(newError(
			sfmt("HTML void element '%v' cannot have closing tag", tagName),
			pos))
	}
	return &Token{Type: EndTagToken, Data: tagName, Pos: pos}
}

func (s *Scanner) scanRawContent(rawTag string) *Token {
	var buf bytes.Buffer

	for {
		ch := s.read()
		pos := s.pos()

		switch ch {
		case eof:
			return s.setError(unexpected(ch, s.pos()))
		case '<':
			next := s.read()
			if next == '/' {
				endTag := s.scanName()
				if endTag == rawTag {
					nch := s.read()
					if nch == '>' {
						// raw tag ends
						s.nextToken = &Token{Type: EndTagToken, Data: endTag, Pos: pos}
						return &Token{Type: TextToken, Data: buf.String()}
					} else {
						s.unread(1)
					}
				}

				buf.WriteString("</" + endTag)
				continue
			} else {
				s.unread(1)
			}
		}

		buf.WriteRune(ch)
	}
}

func (s *Scanner) scan() *Token {
	tt := s.nextTokType
	s.nextTokType = 0

	if s.nextToken != nil {
		tok := s.nextToken
		s.nextToken = nil
		return tok
	}

	if s.inRawTag != "" {
		tok := s.scanRawContent(s.inRawTag)
		s.inRawTag = ""
		return tok
	}

	switch tt {
	case StartTagToken:
		tok := s.scanStartTag()
		if rawTags[tok.Data] {
			s.inRawTag = tok.Data
		}

		return tok

	case EndTagToken:
		return s.scanEndTag()

	case MustacheToken:
		pos := s.pos()
		m, err := s.scanMustache()
		if err != nil {
			return s.setError(err)
		}

		return &Token{Type: MustacheToken, Data: m, Pos: pos}

	case CommentToken:
		return s.scanComment()

	case EOFToken:
		return &Token{Type: EOFToken}
	}

	// scan text
	var tbuf bytes.Buffer

	for {
		// Read next code point.
		ch := s.read()
		pos := s.pos()
		// Check against individual code points
		switch ch {
		case '\n':
			s.skipWhitespace()
			continue

		case eof:
			s.nextTokType = EOFToken

		case '<':
			ch := s.read()
			switch {
			case ch == '/':
				s.nextTokType = EndTagToken
			case isNameStart(ch):
				s.unread(1)
				s.nextTokType = StartTagToken
			case ch == '!':
				nx1 := s.read()
				nx2 := s.read()
				if nx1 == '-' && nx2 == '-' {
					s.nextTokType = CommentToken
				} else {
					s.unread(2)
				}
			default:
				s.unread(1)
			}

		case '{':
			ch := s.read()
			if ch == '{' {
				s.nextTokType = MustacheToken
			} else {
				s.unread(1)
			}
		}

		if s.nextTokType != 0 {
			if tbuf.Len() > 0 {
				return &Token{
					Type: TextToken,
					Data: string(unescape(tbuf.Bytes(), false)),
					Pos:  pos,
				}
			} else {
				return s.scan()
			}
		}

		tbuf.WriteRune(ch)
	}
}

// unscan buffers the previous scan.
func (s *Scanner) unscan() {
	s.tokbufn = true
}

// Current returns the current token.
func (s *Scanner) current() *Token {
	return s.tokbuf
}

// scanComment consumes all characters up to "-->", inclusive.
// This function assumes that the initial "<!--" have just been consumed.
func (s *Scanner) scanComment() *Token {
	var buf bytes.Buffer
	startPos := s.pos()

	for {
		ch0 := s.read()
		if ch0 == eof {
			break
		} else if ch0 == '-' {
			ch1 := s.read()
			ch2 := s.read()
			if ch1 == '-' && ch2 == '>' {
				break
			} else {
				s.unread(2)
			}
		}

		buf.WriteRune(ch0)
	}

	return &Token{Type: CommentToken, Data: buf.String(), Pos: startPos}
}

// scanName consumes a name.
func (s *Scanner) scanName() string {
	var buf bytes.Buffer
	for {
		if ch := s.read(); isName(ch) {
			buf.WriteRune(ch)
		} else {
			s.unread(1)
			return buf.String()
		}
	}
}

// scanWhitespace skips the current code point and all subsequent whitespace.
func (s *Scanner) skipWhitespace() {
	for {
		ch := s.read()
		if ch == eof {
			break
		} else if !isWhitespace(ch) {
			s.unread(1)
			break
		}
	}
}

// This function will initially check for any characters that have been pushed
// back onto the lookahead buffer and return those. Otherwise it will read from
// the reader and do preprocessing to convert newline characters and NULL.
func (s *Scanner) read() rune {
	// If we have runes on our internal lookahead buffer then return those.
	if s.bufn > 0 {
		s.bufi = ((s.bufi + 1) % len(s.buf))
		s.bufn--
		return s.buf[s.bufi]
	}

	// Otherwise read from the reader.
	ch, _, err := s.rd.ReadRune()
	pos := s.pos()
	if err != nil {
		ch = eof
	} else {
		// Preprocess the input stream by replacing FF with LF
		if ch == '\f' {
			ch = '\n'
		}

		// Preprocess the input stream by replacing CR and CRLF with LF
		if ch == '\r' {
			if ch, _, err := s.rd.ReadRune(); err != nil {
				// nop
			} else if ch != '\n' {
				s.unread(1)
			}
			ch = '\n'
		}

		// Track scanner position.
		if ch == '\n' {
			pos.Line++
			pos.Char = 0
		} else {
			pos.Char++
		}
	}

	// Add to circular buffer.
	s.bufi = ((s.bufi + 1) % len(s.buf))
	s.buf[s.bufi] = ch
	s.bufpos[s.bufi] = pos
	return ch
}

// unread adds the previous n code points back onto the buffer.
func (s *Scanner) unread(n int) {
	for i := 0; i < n; i++ {
		s.bufi = ((s.bufi + len(s.buf) - 1) % len(s.buf))
		s.bufn++
	}
}

// curr reads the current code point.
func (s *Scanner) curr() rune {
	return s.buf[s.bufi]
}

// Pos reads the current position of the scanner.
func (s *Scanner) pos() Pos {
	return s.bufpos[s.bufi]
}

// isWhitespace returns true if the rune is a space, tab, or newline.
func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n'
}

// isLetter returns true if the rune is a letter.
func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

// isDigit returns true if the rune is a digit.
func isDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9')
}

// isNameStart returns true if the rune can start a name.
func isNameStart(ch rune) bool {
	return isLetter(ch) || ch == '_'
}

// isName returns true if the character is a tag name code point.
func isName(ch rune) bool {
	return isNameStart(ch) || ch == '-' || ch == ':' || ch == '.'
}
