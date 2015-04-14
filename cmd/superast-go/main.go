package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"

	"github.com/mvdan/superast"
)

var (
	pretty = flag.Bool("p", false, "indent (pretty print) output")
)

func main() {
	flag.Parse()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "stdin.go", os.Stdin, 0)
	if err != nil {
		log.Fatal(err)
	}
	a := superast.NewAST(fset)
	ast.Walk(a, f)

	if *pretty {
		b, err := json.Marshal(a.RootBlock)
		if err != nil {
			log.Fatal(err)
		}
		var out bytes.Buffer
		if err := json.Indent(&out, b, "", "  "); err != nil {
			log.Fatal(err)
		}
		if _, err := out.WriteTo(os.Stdout); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\n")
	} else {
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(a.RootBlock); err != nil {
			log.Println(err)
		}
	}

}
