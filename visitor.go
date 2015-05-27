package superast

import (
	"go/ast"
	"go/token"
	"log"
	"strconv"
)

var allowedImports = map[string]struct{}{
	"fmt": struct{}{},
}

type AST struct {
	curID      int
	RootBlock  *block
	nodeStack  []ast.Node
	stmtsStack []*[]stmt
	fset       *token.FileSet
	position   token.Position
}

func NewAST(fset *token.FileSet) *AST {
	a := &AST{
		fset: fset,
	}
	a.RootBlock = &block{
		id:    a.newID(),
		Stmts: make([]stmt, 0),
	}
	a.pushStmts(&a.RootBlock.Stmts)
	return a
}

func (a *AST) newID() id {
	i := a.curID
	a.curID++
	return id{ID: i}
}

func (a *AST) pos() pos {
	return pos{Line: a.position.Line, Col: a.position.Column}
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

func exprString(x ast.Expr) string {
	switch t := x.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.BasicLit:
		return t.Value
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return exprString(t.X)
	default:
		log.Printf("exprString: %#v", x)
	}
	return ""
}

func exprType(x ast.Expr) *dataType {
	switch x.(type) {
	default:
		log.Printf("%#v", x)
	}
	return nil
}

var funcNames = map[string]string{
	"fmt.Print":   "print",
	"fmt.Println": "print",
	"println":     "print",
}

type field struct {
	vName string
	dType *dataType
}

func flattenFieldList(fieldList *ast.FieldList) []field {
	if fieldList == nil {
		return nil
	}
	var fields []field
	for _, f := range fieldList.List {
		t := exprType(f.Type)
		if len(f.Names) == 0 {
			fields = append(fields, field{
				vName:  "",
				dType: t,
			})
		}
		for _, n := range f.Names {
			fields = append(fields, field{
				vName:  n.Name,
				dType: t,
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

var zeroValues = map[string]value{
	"int":    0,
	"double": 0.0,
	"char":   `'\0'`,
	"string": "",
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
	a.position = a.fset.Position(node.Pos())
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
			n = exprString(x.Name)
		}
		switch t := x.Type.(type) {
		case *ast.StructType:
			decl := &structDecl{
				id:   a.newID(),
				pos:  a.pos(),
				Type: "struct-declaration",
				Name: n,
			}
			for _, f := range flattenFieldList(t.Fields) {
				attr := varDecl{
					id:   a.newID(),
					pos:  a.pos(),
					Type: "variable-declaration",
					Name: f.vName,
					DataType: f.dType,
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
			id:    a.newID(),
			pos:   a.pos(),
			Type:  "identifier",
			Value: x.Name,
		}
		a.addStmt(id)
	case *ast.BasicLit:
		lit := &identifier{
			id:    a.newID(),
			pos:   a.pos(),
			Type:  "string",
			Value: strUnquote(x.Value),
		}
		a.addStmt(lit)
	case *ast.CallExpr:
		name := exprString(x.Fun)
		if newname, e := funcNames[name]; e {
			name = newname
		}
		call := &funcCall{
			id:   a.newID(),
			pos:  a.pos(),
			Type: "function-call",
			Name: name,
		}
		a.addStmt(call)
		a.pushStmts(&call.Args)
	case *ast.FuncDecl:
		name := x.Name.Name
		var retType *dataType
		results := flattenFieldList(x.Type.Results)
		switch len(results) {
		case 1:
			retType = results[0].dType
		}
		fn := &funcDecl{
			id:   a.newID(),
			pos:  a.pos(),
			Type: "function-declaration",
			Name: name,
			RetType: retType,
			Block: &block{
				id:    a.newID(),
				Stmts: make([]stmt, 0),
			},
		}
		for _, f := range flattenFieldList(x.Type.Params) {
			param := varDecl{
				id:   a.newID(),
				pos:  a.pos(),
				Type: "variable-declaration",
				Name: f.vName,
				DataType: f.dType,
			}
			fn.Params = append(fn.Params, param)
		}
		a.addStmt(fn)
		a.pushStmts(&fn.Block.Stmts)
	case *ast.DeclStmt:
		gd, _ := x.Decl.(*ast.GenDecl)
		for _, spec := range gd.Specs {
			switch s := spec.(type) {
			case *ast.ValueSpec:
				t := exprString(s.Type)
				for i, id := range s.Names {
					n := exprString(id)
					v, _ := zeroValues[t]
					if s.Values != nil {
						v = exprString(s.Values[i])
					}
					decl := &varDecl{
						id:   a.newID(),
						pos:  a.pos(),
						Type: "variable-declaration",
						Name: n,
						DataType: &dataType{
							id:   a.newID(),
							Name: t,
						},
						Init: &identifier{
							id:    a.newID(),
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
			n := exprString(expr)
			var v value
			var t string
			switch r := x.Rhs[i].(type) {
			case *ast.BasicLit:
				t, _ = basicLitName[r.Kind]
				v = strUnquote(r.Value)
			case *ast.CompositeLit:
				t = exprString(r.Type)
			default:
			}
			asg := &varDecl{
				id:   a.newID(),
				pos:  a.pos(),
				Type: "variable-declaration",
				Name: n,
				DataType: &dataType{
					id:   a.newID(),
					Name: t,
				},
				Init: &identifier{
					id:    a.newID(),
					Type:  t,
					Value: v,
				},
			}
			a.addStmt(asg)
		}
		return nil
	case *ast.BlockStmt:
	case *ast.ExprStmt:
	case *ast.GenDecl:
	default:
		return nil
	}
	a.pushNode(node)
	return a
}
