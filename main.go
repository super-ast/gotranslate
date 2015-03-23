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
	Stmts []statement `json:"statements"`
}

type dataType struct {
	Name string `json:"name"`
}

type parameter struct {
	Name     string   `json:"name"`
	DataType dataType `json:"data-type"`
}

type statement struct {
	Line    int         `json:"line"`
	Type    string      `json:"type"`
	Name    string      `json:"name"`
	RetType *dataType   `json:"return-type,omitempty"`
	Params  []parameter `json:"parameters,omitempty"`
	Args    []statement `json:"arguments,omitempty"`
	Left    *statement  `json:"left,omitempty"`
	Right   *statement  `json:"right,omitempty"`
	Block   *block      `json:"block,omitempty"`
}

type superAST struct {
	RootBlock  *block
	nodeStack  []ast.Node
	stmtsStack []*[]statement
	fset       *token.FileSet
}

func newSuperAST(fset *token.FileSet) *superAST {
	a := &superAST{
		fset:      fset,
		RootBlock: new(block),
	}
	a.stmtsStack = append(a.stmtsStack, &a.RootBlock.Stmts)
	return a
}

func (a *superAST) pushNode(node ast.Node) {
	a.nodeStack = append(a.nodeStack, node)
}

func (a *superAST) curNode() ast.Node {
	return a.nodeStack[len(a.nodeStack)-1]
}

func (a *superAST) popNode() {
	a.nodeStack = a.nodeStack[:len(a.nodeStack)-1]
}

func (a *superAST) pushStmts(stmts *[]statement) {
	a.stmtsStack = append(a.stmtsStack, stmts)
}

func (a *superAST) curStmts() *[]statement {
	return a.stmtsStack[len(a.stmtsStack)-1]
}

func (a *superAST) popStmts() {
	a.stmtsStack = a.stmtsStack[:len(a.stmtsStack)-1]
}

func (a *superAST) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		switch a.curNode().(type) {
		case *ast.CallExpr:
			a.popStmts()
		case *ast.FuncDecl:
			a.popStmts()
		}
		a.popNode()
		log.Printf("%s}", strings.Repeat("  ", len(a.nodeStack)))
		return nil
	}
	curStmts := a.curStmts()
	pos := a.fset.Position(node.Pos())
	log.Printf("%s%T - %#v", strings.Repeat("  ", len(a.nodeStack)), node, pos)
	switch x := node.(type) {
	case *ast.BasicLit:
	case *ast.BlockStmt:
	case *ast.CallExpr:
		call := statement{
			Line: pos.Line,
			Type: "function-call",
			Name: "print",
		}
		*curStmts = append(*curStmts, call)
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
		fn := statement{
			Line: pos.Line,
			Type: "function-declaration",
			Name: name,
			RetType: &dataType{
				Name: "int",
			},
			Block: new(block),
		}
		*curStmts = append(*curStmts, fn)
		a.pushStmts(&fn.Block.Stmts)
	case *ast.FuncType:
	case *ast.GenDecl:
	case *ast.Ident:
	case *ast.ImportSpec:
	case *ast.SelectorExpr:
	default:
		log.Printf("Uncatched ast.Node type: %T\n", node)
	}
	a.pushNode(node)
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
