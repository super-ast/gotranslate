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
	"strconv"
	"strings"
)

var (
	pretty = flag.Bool("p", false, "indent (pretty print) output")
)

var allowedImports = map[string]struct{}{
	"fmt": struct{}{},
	"log": struct{}{},
}

type block struct {
	Statements []statement `json:"statements"`
}

type dataType struct {
	Name string `json:"name"`
}

type parameter struct {
	Name     string   `json:"name"`
	DataType dataType `json:"data-type"`
}

type statement struct {
	Line       int         `json:"line"`
	Type       string      `json:"type"`
	Name       string      `json:"name"`
	ReturnType *dataType   `json:"return-type,omitempty"`
	Parameters []parameter `json:"parameters,omitempty"`
	Arguments  []statement `json:"arguments,omitempty"`
	Left       *statement  `json:"left,omitempty"`
	Right      *statement  `json:"right,omitempty"`
	Block      *block      `json:"block,omitempty"`
}

type superAST struct {
	nodeStack  []ast.Node
	blockStack []*block
	fset       *token.FileSet
}

func newSuperAST(fset *token.FileSet) *superAST {
	a := &superAST{
		fset: fset,
	}
	a.blockStack = append(a.blockStack, new(block))
	return a
}

func (a *superAST) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		popNode := a.nodeStack[len(a.nodeStack)-1]
		switch popNode.(type) {
		case *ast.FuncDecl:
			a.blockStack = a.blockStack[:len(a.blockStack)-1]
		}
		a.nodeStack = a.nodeStack[:len(a.nodeStack)-1]
		log.Printf("%s}", strings.Repeat("  ", len(a.nodeStack)))
		return nil
	}
	curBlock := a.blockStack[len(a.blockStack)-1]
	pos := a.fset.Position(node.Pos())
	log.Printf("%s%T - %#v", strings.Repeat("  ", len(a.nodeStack)), node, pos)
	switch x := node.(type) {
	case *ast.BasicLit:
	case *ast.BlockStmt:
	case *ast.CallExpr:
		call := statement{
			Line:      pos.Line,
			Type:      "function-call",
			Name:      "print",
			Arguments: make([]statement, 0),
		}
		curBlock.Statements = append(curBlock.Statements, call)
	case *ast.ExprStmt:
	case *ast.FieldList:
	case *ast.File:
		pname := x.Name.Name
		if pname != "main" {
			log.Fatalf(`Package name is not "main": "%s"`, pname)
		}
		imports := x.Imports
		for _, imp := range imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				log.Fatalf("Error when unquoting import: %s", err)
			}
			if _, e := allowedImports[path]; !e {
				log.Fatalf(`Import path not allowed: "%s"`, path)
			}
		}
	case *ast.FuncDecl:
		name := x.Name.Name
		/*var params, results []*ast.Field
		if x.Type.Params != nil {
			params = x.Type.Params.List
		}
		if x.Type.Results != nil {
			results = x.Type.Results.List
		}*/
		function := statement{
			Line: pos.Line,
			Type: "function-declaration",
			Name: name,
			ReturnType: &dataType{
				Name: "int",
			},
			Parameters: make([]parameter, 0),
			Block:      new(block),
		}
		curBlock.Statements = append(curBlock.Statements, function)
		a.blockStack = append(a.blockStack, function.Block)
	case *ast.FuncType:
	case *ast.GenDecl:
	case *ast.Ident:
	case *ast.ImportSpec:
	case *ast.SelectorExpr:
	default:
		log.Printf("Uncatched ast.Node type: %T\n", node)
	}
	a.nodeStack = append(a.nodeStack, node)
	return a
}

func main() {
	flag.Parse()
	fset := token.NewFileSet()
	src := `
package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	f, err := parser.ParseFile(fset, "hello_world.go", src, 0)
	if err != nil {
		log.Fatal(err)
	}
	a := newSuperAST(fset)
	ast.Walk(a, f)

	rootBlock := a.blockStack[0]
	if *pretty {
		b, err := json.Marshal(rootBlock)
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
		if err := enc.Encode(rootBlock); err != nil {
			log.Println(err)
		}
	}

}
