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
	Id    int         `json:"id"`
	Stmts []statement `json:"statements"`
}

type dataType struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type parameter struct {
	Name     string   `json:"name"`
	DataType dataType `json:"data-type"`
}

type statement struct {
	Id      int         `json:"id"`
	Line    int         `json:"line"`
	Type    string      `json:"type"`
	Name    string      `json:"name,omitempty"`
	Value   string      `json:"value,omitempty"`
	RetType *dataType   `json:"return-type,omitempty"`
	Params  []parameter `json:"parameters,omitempty"`
	Args    []statement `json:"arguments,omitempty"`
	Left    *statement  `json:"left,omitempty"`
	Right   *statement  `json:"right,omitempty"`
	Block   *block      `json:"block,omitempty"`
}

type superAST struct {
	curID      int
	RootBlock  *block
	nodeStack  []ast.Node
	stmtsStack []*[]statement
	fset       *token.FileSet
}

func newSuperAST(fset *token.FileSet) *superAST {
	a := &superAST{
		curID: 1,
		fset:  fset,
		RootBlock: &block{
			Id: 0,
		},
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

func strUnquote(s string) string {
	u, err := strconv.Unquote(s)
	if err != nil {
		log.Fatalf("Error when unquoting string: %s", err)
	}
	return u
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
		lit := statement{
			Id:    a.curID,
			Line:  pos.Line,
			Type:  "string",
			Value: strUnquote(x.Value),
		}
		a.curID++
		*curStmts = append(*curStmts, lit)
	case *ast.CallExpr:
		call := statement{
			Id:   a.curID,
			Line: pos.Line,
			Type: "function-call",
			Name: "print",
			Args: make([]statement, 0),
		}
		a.curID++
		*curStmts = append(*curStmts, call)
		a.pushStmts(&call.Args)
	case *ast.File:
		pname := x.Name.Name
		if pname != "main" {
			log.Fatalf(`Package name is not "main": "%s"`, pname)
		}
		imports := x.Imports
		for _, imp := range imports {
			path := strUnquote(imp.Path.Value)
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
			Id:    a.curID,
			Line:  pos.Line,
			Type:  "function-declaration",
			Name:  name,
			Block: new(block),
		}
		a.curID++
		fn.RetType = &dataType{
			Id:   a.curID,
			Name: "int",
		}
		a.curID++
		*curStmts = append(*curStmts, fn)
		a.pushStmts(&fn.Block.Stmts)
	case *ast.BlockStmt:
	case *ast.ExprStmt:
	case *ast.FieldList:
	case *ast.FuncType:
	case *ast.GenDecl:
	case *ast.Ident:
	case *ast.SelectorExpr:
	default:
		log.Printf("Ignoring %T\n", node)
		return nil
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
