package main

import (
	"bytes"

	"github.com/gowade/whtml"
	pp "github.com/tonnerre/golang-pretty"
)

func main() {
	buf := bytes.NewBufferString(`<div>
	<div hidden={{this.Hidden}} Enabled></div>
	<fk.Fck xxx:ren="aa" ...{{this.Attrs()}} />
</div>`)

	s := whtml.NewScanner(buf)
	for {
		tok := s.Scan()
		if tok.Type == whtml.EOFToken {
			break
		}

		pp.Println(tok)
		if tok.Type == whtml.ErrorToken {
			println(s.Errors.Error())
			break
		}
	}
}
