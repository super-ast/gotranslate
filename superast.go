package superast

import (
	"go/ast"
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
	ID    int    `json:"id"`
	Stmts []stmt `json:"statements"`
}

type dataType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type varDecl struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	DataType *dataType `json:"data-type,omitempty"`
}

type stmt interface{}

type statement struct {
	ID       int        `json:"id"`
	Line     int        `json:"line"`
	Type     string     `json:"type"`
	Name     string     `json:"name,omitempty"`
	Value    string     `json:"value,omitempty"`
	DataType *dataType  `json:"data-type,omitempty"`
	RetType  *dataType  `json:"return-type,omitempty"`
	Params   []varDecl  `json:"parameters,omitempty"`
	Args     []stmt     `json:"arguments,omitempty"`
	Init     *statement `json:"init,omitempty"`
	Left     *statement `json:"left,omitempty"`
	Right    *statement `json:"right,omitempty"`
	Block    *block     `json:"block,omitempty"`
}

type identifier struct {
	ID       int    `json:"id"`
	Line     int    `json:"line"`
	Type     string `json:"type"`
	Value    string `json:"value"`
}

type structDecl struct {
	ID    int       `json:"id"`
	Line  int       `json:"line"`
	Type  string    `json:"type"`
	Name  string    `json:"name"`
	Attrs []varDecl `json:"attributes"`
}

type AST struct {
	curID      int
	RootBlock  *block
	nodeStack  []ast.Node
	stmtsStack []*[]stmt
	fset       *token.FileSet
}

func NewAST(fset *token.FileSet) *AST {
	a := &AST{
		curID: 1,
		fset:  fset,
		RootBlock: &block{
			ID:    0,
			Stmts: make([]stmt, 0),
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

func (a *AST) pushStmts(stmts *[]stmt) {
	a.stmtsStack = append(a.stmtsStack, stmts)
}

func (a *AST) addStmt(s stmt) {
	if len(a.stmtsStack) == 0 {
		return
	}
	curStmts := a.stmtsStack[len(a.stmtsStack)-1]
	*curStmts = append(*curStmts, s)
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
		return s
	}
	return u
}

func exprToString(x ast.Expr) string {
	switch t := x.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.BasicLit:
		return t.Value
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return exprToString(t.X)
	default:
		log.Printf("foo %T\n", t)
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

var basicLitName = map[token.Token]string{
	token.INT:    "int",
	token.FLOAT:  "double",
	token.CHAR:   "char",
	token.STRING: "string",
}

var zeroValues = map[string]string{
	"int":    "0",
	"double": "0.0",
	"char":   `'\0'`,
	"string": `""`,
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
	case *ast.TypeSpec:
		n := ""
		if x.Name != nil {
			n = exprToString(x.Name)
		}
		switch t := x.Type.(type) {
		case *ast.StructType:
			decl := &structDecl{
				ID:   a.newID(),
				Line: pos.Line,
				Type: "struct-declaration",
				Name: n,
			}
			for _, f := range flattenFieldList(t.Fields) {
				attr := varDecl{
					ID:   a.newID(),
					Name: f.varName,
					DataType: &dataType{
						ID:   a.newID(),
						Name: f.typeName,
					},
				}
				decl.Attrs = append(decl.Attrs, attr)
			}
			a.addStmt(decl)
		}
	case *ast.Ident:
		switch parentNode.(type) {
		case *ast.CallExpr:
		default:
			return nil
		}
		id := &identifier{
			ID:    a.newID(),
			Line:  pos.Line,
			Type:  "identifier",
			Value: x.Name,
		}
		a.addStmt(id)
	case *ast.BasicLit:
		lit := &statement{
			ID:    a.newID(),
			Line:  pos.Line,
			Type:  "string",
			Value: strUnquote(x.Value),
		}
		a.addStmt(lit)
	case *ast.CallExpr:
		name := exprToString(x.Fun)
		if newname, e := funcNames[name]; e {
			name = newname
		}
		call := &statement{
			ID:   a.newID(),
			Line: pos.Line,
			Type: "function-call",
			Name: name,
		}
		a.addStmt(call)
		a.pushStmts(&call.Args)
	case *ast.FuncDecl:
		name := x.Name.Name
		fn := &statement{
			ID:   a.newID(),
			Line: pos.Line,
			Type: "function-declaration",
			Name: name,
			RetType: &dataType{
				ID: a.newID(),
			},
			Block: &block{
				ID:    a.newID(),
				Stmts: make([]stmt, 0),
			},
		}
		for _, f := range flattenFieldList(x.Type.Params) {
			param := varDecl{
				ID:   a.newID(),
				Name: f.varName,
				DataType: &dataType{
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
		case 1:
			fn.RetType.Name = results[0].typeName
		}
		a.addStmt(fn)
		a.pushStmts(&fn.Block.Stmts)
	case *ast.DeclStmt:
		log.Printf("%#v", x.Decl)
		gd, _ := x.Decl.(*ast.GenDecl)
		for _, spec := range gd.Specs {
			switch s := spec.(type) {
			case *ast.ValueSpec:
				t := exprToString(s.Type)
				for i, id := range s.Names {
					n := exprToString(id)
					v, _ := zeroValues[t]
					if s.Values != nil {
						v = exprToString(s.Values[i])
					}
					decl := &statement{
						ID:   a.newID(),
						Line: pos.Line,
						Type: "variable-declaration",
						Name: n,
						DataType: &dataType{
							ID:   a.newID(),
							Name: t,
						},
						Init: &statement{
							ID:    a.newID(),
							Type:  t,
							Value: v,
						},
					}
					a.addStmt(decl)
				}
			}
		}
	case *ast.AssignStmt:
		for i, expr := range x.Lhs {
			n := exprToString(expr)
			l, _ := x.Rhs[i].(*ast.BasicLit)
			value := strUnquote(l.Value)
			typeName, _ := basicLitName[l.Kind]
			asg := &statement{
				ID:   a.newID(),
				Line: pos.Line,
				Type: "variable-declaration",
				Name: n,
				DataType: &dataType{
					ID:   a.newID(),
					Name: typeName,
				},
				Init: &statement{
					ID:    a.newID(),
					Type:  typeName,
					Value: value,
				},
			}
			a.addStmt(asg)
		}
		return nil
	case *ast.BlockStmt:
	case *ast.ExprStmt:
	case *ast.FieldList:
	case *ast.GenDecl:
	case *ast.SelectorExpr:
	default:
		log.Printf("Ignoring %T\n", node)
		return nil
	}
	a.pushNode(node)
	return a
}
