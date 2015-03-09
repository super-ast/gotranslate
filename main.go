package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log"
)

type superAST struct {
}

func (a superAST) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}
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
	case *ast.FuncDecl:
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
	var a superAST
	ast.Walk(a, f)
}
