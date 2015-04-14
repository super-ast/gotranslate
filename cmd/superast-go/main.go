package main

import (
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

	var err error
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "stdin.go", os.Stdin, 0)
	if err != nil {
		log.Fatal(err)
	}
	a := superast.NewAST(fset)
	ast.Walk(a, f)

	var b []byte
	if *pretty {
		b, err = json.MarshalIndent(a.RootBlock, "", "  ")
	} else {
		b, err = json.Marshal(a.RootBlock)
	}
	if err != nil {
		log.Fatal(err)
	}
	if _, err = os.Stdout.Write(b); err != nil {
		log.Fatal(err)
	}
	fmt.Println()

}
