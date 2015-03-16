package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strconv"
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
	ReturnType dataType    `json:"return-type"`
	Parameters []parameter `json:"parameters"`
	Block      block       `json:"block"`
}

type superAST struct {
	fset      *token.FileSet
	rootBlock block
}

func (a *superAST) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}
	pos := a.fset.Position(node.Pos())
	log.Printf("%#v", pos)
	log.Printf("%T", node)
	switch x := node.(type) {
	case *ast.BasicLit:
	case *ast.BlockStmt:
	case *ast.CallExpr:
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
		var params, results []*ast.Field
		if x.Type.Params != nil {
			params = x.Type.Params.List
		}
		if x.Type.Results != nil {
			results = x.Type.Results.List
		}
		log.Printf("func %s %v %v", name, params, results)
		function := statement{
			Line: pos.Line,
			Type: "function-call",
			Name: name,
			ReturnType: dataType{
				Name: "int",
			},
			Parameters: nil,
		}
		a.rootBlock.Statements = append(a.rootBlock.Statements, function)
	case *ast.FuncType:
	case *ast.GenDecl:
	case *ast.Ident:
	case *ast.ImportSpec:
	case *ast.SelectorExpr:
	default:
		log.Printf("Uncatched ast.Node type: %T\n", node)
	}
	return a
}

func main() {
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
	a := superAST{
		fset: fset,
	}
	ast.Walk(&a, f)
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(&a.rootBlock); err != nil {
		log.Println(err)
	}
}
