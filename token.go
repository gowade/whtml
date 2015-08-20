package whtml

type AttributeType uint32

const (
	// attribute="Value"
	StringAttribute AttributeType = iota

	// attribute={{this.Value}}
	MustacheAttribute

	// attribute without an equal assign, treated as boolean true
	BoolAttribute

	// ...{{this.vdomAttrs()}}
	VariadicAttribute
)

type Attribute struct {
	Type      AttributeType
	Key, Val  string
	Pos       Pos
	Mustaches []string // interpolated mustaches in StringAttribute
}

// Token represents a HTML Token
type Token struct {
	Type  TokenType
	Data  string
	Attrs []Attribute
	Pos   Pos
}

// A TokenType is the type of a Token.
type TokenType uint32

const (
	// ErrorToken means that an error occurred during tokenization.
	ErrorToken TokenType = iota

	// EOF
	EOFToken

	// TextToken means a text node.
	TextToken

	// Mustache token
	MustacheToken

	// A StartTagToken looks like <a>.
	StartTagToken

	// An EndTagToken looks like </a>.
	EndTagToken

	// A SelfClosingTagToken tag looks like <br/>.
	SelfClosingTagToken

	// A CommentToken looks like <!--x-->.
	CommentToken
)
