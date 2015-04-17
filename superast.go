package superast

import (
	"go/ast"
	"go/token"
	"log"
	"strconv"
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

func NewAST(fset *token.FileSet) *AST {
	a := &AST{
		curID: 1,
		fset:  fset,
		RootBlock: &block{
			ID: 0,
		},
	}
	a.pushStmts(&a.RootBlock.Stmts)
	return a
}

func (a *AST) newID() int {
	i := a.curID
	a.curID++
	return i
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

func (a *AST) newStmt() *statement {
	if len(a.stmtsStack) == 0 {
		return nil
	}
	curStmts := a.stmtsStack[len(a.stmtsStack)-1]
	*curStmts = append(*curStmts, statement{})
	list := *curStmts
	return &list[len(list)-1]
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
	if fieldList == nil {
		return nil
	}
	var fields []field
	for _, f := range fieldList.List {
		t := exprToString(f.Type)
		if len(f.Names) == 0 {
			fields = append(fields, field{
				varName:  "",
				typeName: t,
			})
		}
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
		lit := a.newStmt()
		*lit = statement{
			ID:    a.newID(),
			Line:  pos.Line,
			Type:  "string",
			Value: strUnquote(x.Value),
		}
	case *ast.CallExpr:
		name := exprToString(x.Fun)
		if newname, e := funcNames[name]; e {
			name = newname
		}
		call := a.newStmt()
		*call = statement{
			ID:   a.newID(),
			Line: pos.Line,
			Type: "function-call",
			Name: name,
		}
		a.pushStmts(&call.Args)
	case *ast.FuncDecl:
		name := x.Name.Name
		fn := a.newStmt()
		*fn = statement{
			ID:   a.newID(),
			Line: pos.Line,
			Type: "function-declaration",
			Name: name,
			RetType: &dataType{
				ID: a.newID(),
			},
			Block: &block{
				ID: a.newID(),
			},
		}
		for _, f := range flattenFieldList(x.Type.Params) {
			param := parameter{
				ID:   a.newID(),
				Name: f.varName,
				DataType: dataType{
					ID:   a.newID(),
					Name: f.typeName,
				},
			}
			fn.Params = append(fn.Params, param)
		}
		results := flattenFieldList(x.Type.Results)
		switch len(results) {
		case 0:
			fn.RetType.Name = "void"
			if name == "main" {
				fn.RetType.Name = "int"
			}
		case 1:
			fn.RetType.Name = results[0].typeName
		}
		a.pushStmts(&fn.Block.Stmts)
	case *ast.BlockStmt:
	case *ast.ExprStmt:
	case *ast.FieldList:
	case *ast.GenDecl:
	case *ast.Ident:
	case *ast.SelectorExpr:
	default:
		return nil
	}
	a.pushNode(node)
	return a
}
