package main

import (
	"bytes"
	"os"

	"github.com/gowade/whtml"
	pp "github.com/tonnerre/golang-pretty"
)

var (
	buf = bytes.NewBufferString(`<div t="a &lt b">
	<!-- sldflowoef o o ae-> -->
	<div hidden={{this.Hidden}} Enabled><div>&nbsp; ksdkfk</div></div>
	<br><fk.Fck xxx:ren="aa" ...{{this.Attrs()}} />
</div>`)
)

func testToken() {
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

func main() {
	nodes, err := whtml.Parse(buf)
	if err != nil {
		println("ERROR", err.Error())
	}

	for _, node := range nodes {
		node.Render(os.Stdout)
	}
}
