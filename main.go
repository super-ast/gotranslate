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
	ID    int         `json:"id"`
	Stmts []statement `json:"statements"`
}

type dataType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type parameter struct {
	Name     string   `json:"name"`
	DataType dataType `json:"data-type"`
}

type statement struct {
	ID      int         `json:"id"`
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
			ID:    0,
			Stmts: make([]statement, 0),
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

func fieldName(x ast.Expr) *ast.Ident {
	switch t := x.(type) {
	case *ast.Ident:
		return t
	case *ast.SelectorExpr:
		if _, ok := t.X.(*ast.Ident); ok {
			return t.Sel
		}
	case *ast.StarExpr:
		return fieldName(t.X)
	}
	return nil
}

func callName(x ast.Expr) string {
	switch t := x.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return callName(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return callName(t.X)
	}
	return ""
}

var funcNames = map[string]string{
	"fmt.Println": "print",
	"println":     "print",
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
		return nil
	}
	curStmts := a.curStmts()
	pos := a.fset.Position(node.Pos())
	log.Printf("%s%T - %#v", strings.Repeat("  ", len(a.nodeStack)), node, pos)
	switch x := node.(type) {
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
	case *ast.BasicLit:
		lit := statement{
			ID:    a.curID,
			Line:  pos.Line,
			Type:  "string",
			Value: strUnquote(x.Value),
		}
		a.curID++
		*curStmts = append(*curStmts, lit)
	case *ast.CallExpr:
		name := callName(x.Fun)
		if newname, e := funcNames[name]; e {
			name = newname
		}
		call := statement{
			ID:   a.curID,
			Line: pos.Line,
			Type: "function-call",
			Name: name,
		}
		a.curID++
		*curStmts = append(*curStmts, call)
		a.pushStmts(&call.Args)
	case *ast.FuncDecl:
		name := x.Name.Name
		fn := statement{
			ID:   a.curID,
			Line: pos.Line,
			Type: "function-declaration",
			Name: name,
		}
		a.curID++
		fn.RetType = &dataType{
			ID: a.curID,
		}
		a.curID++
		//params := x.Type.Params
		results := x.Type.Results
		switch results.NumFields() {
		case 0:
			fn.RetType.Name = "void"
			if name == "main" {
				fn.RetType.Name = "int"
			}
		case 1:
			if i := fieldName(results.List[0].Type); i == nil {
				fn.RetType.Name = ""
			} else {
				fn.RetType.Name = i.Name
			}
		}
		fn.Block = &block{
			ID:    a.curID,
			Stmts: make([]statement, 0),
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
