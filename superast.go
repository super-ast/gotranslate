package superast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"strconv"
	"strings"
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
	ID       int      `json:"id"`
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

type AST struct {
	curID      int
	RootBlock  *block
	nodeStack  []ast.Node
	stmtsStack []*[]statement
	fset       *token.FileSet
}

func newAST(fset *token.FileSet) *AST {
	a := &AST{
		curID: 1,
		fset:  fset,
		RootBlock: &block{
			ID:    0,
			Stmts: make([]statement, 0),
		},
	}
	a.pushStmts(&a.RootBlock.Stmts)
	return a
}

func (a *AST) pushNode(node ast.Node) {
	a.nodeStack = append(a.nodeStack, node)
}

func (a *AST) curNode() ast.Node {
	if len(a.nodeStack) == 0 {
		return nil
	}
	return a.nodeStack[len(a.nodeStack)-1]
}

func (a *AST) popNode() {
	if len(a.nodeStack) == 0 {
		return
	}
	a.nodeStack = a.nodeStack[:len(a.nodeStack)-1]
}

func (a *AST) pushStmts(stmts *[]statement) {
	a.stmtsStack = append(a.stmtsStack, stmts)
}

func (a *AST) addStmt(stmt statement) {
	if len(a.stmtsStack) == 0 {
		return
	}
	curStmts := a.stmtsStack[len(a.stmtsStack)-1]
	*curStmts = append(*curStmts, stmt)
}

func (a *AST) popStmts() {
	if len(a.stmtsStack) == 0 {
		return
	}
	a.stmtsStack = a.stmtsStack[:len(a.stmtsStack)-1]
}

func strUnquote(s string) string {
	u, err := strconv.Unquote(s)
	if err != nil {
		log.Fatalf("Error when unquoting string: %s", err)
	}
	return u
}

func exprToString(x ast.Expr) string {
	switch t := x.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return exprToString(t.X)
	}
	return ""
}

var funcNames = map[string]string{
	"fmt.Println": "print",
	"println":     "print",
}

type field struct {
	varName, typeName string
}

func flattenFieldList(fieldList *ast.FieldList) []field {
	var fields []field
	for _, f := range fieldList.List {
		t := exprToString(f.Type)
		for _, n := range f.Names {
			fields = append(fields, field{
				varName:  n.Name,
				typeName: t,
			})
		}
	}
	return fields
}

func (a *AST) Visit(node ast.Node) ast.Visitor {
	parentNode := a.curNode()
	if node == nil {
		switch parentNode.(type) {
		case *ast.CallExpr:
			a.popStmts()
		case *ast.FuncDecl:
			a.popStmts()
		}
		a.popNode()
		return nil
	}
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
		a.addStmt(lit)
	case *ast.CallExpr:
		name := exprToString(x.Fun)
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
		a.addStmt(call)
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
		for _, f := range flattenFieldList(x.Type.Params) {
			param := parameter{
				ID:   a.curID,
				Name: f.varName,
			}
			a.curID++
			param.DataType = dataType{
				ID:   a.curID,
				Name: f.typeName,
			}
			a.curID++
			fn.Params = append(fn.Params, param)
		}
		results := x.Type.Results
		switch results.NumFields() {
		case 0:
			fn.RetType.Name = "void"
			if name == "main" {
				fn.RetType.Name = "int"
			}
		case 1:
			fn.RetType.Name = exprToString(results.List[0].Type)
		}
		fn.Block = &block{
			ID:    a.curID,
			Stmts: make([]statement, 0),
		}
		a.curID++
		a.addStmt(fn)
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

func ParseString(src string) *AST {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "hello_world.go", src, 0)
	if err != nil {
		log.Fatal(err)
	}
	a := newAST(fset)
	ast.Walk(a, f)
	return a
}
