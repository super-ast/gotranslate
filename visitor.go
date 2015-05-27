package superast

import (
	"go/ast"
	"go/token"
	"strconv"
)

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
	}
	return ""
}

func exprType(x ast.Expr) *dataType {
	if s := exprString(x); s != "" {
		return &dataType{
			Name: s,
		}
	}
	switch t := x.(type) {
	case *ast.ArrayType:
		return &dataType{
			Name:    "vector",
			SubType: exprType(t.Elt),
		}
	}
	return nil
}

type namedType struct {
	vName string
	dType *dataType
}

func flattenNames(baseType ast.Expr, names []*ast.Ident) []namedType {
	t := exprType(baseType)
	if len(names) == 0 {
		return []namedType{
			{ vName: "", dType: t, },
		}
	}
	var types []namedType
	for _, n := range names {
		types = append(types, namedType{
			vName: n.Name,
			dType: t,
		})
	}
	return types
}

func flattenFieldList(fieldList *ast.FieldList) []namedType {
	if fieldList == nil {
		return nil
	}
	var types []namedType
	for _, f := range fieldList.List {
		for _, t := range flattenNames(f.Type, f.Names) {
			types = append(types, t)
		}
	}
	return types
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

func (a *AST) assignIdToDataType(dType *dataType) *dataType {
	if dType == nil {
		return nil
	}
	dTypeCopy := *dType
	dTypeCopy.id = a.newID()
	if dTypeCopy.SubType != nil {
		dTypeCopy.SubType = a.assignIdToDataType(dTypeCopy.SubType)
	}
	return &dTypeCopy
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
					id:       a.newID(),
					pos:      a.pos(),
					Type:     "variable-declaration",
					Name:     f.vName,
					DataType: a.assignIdToDataType(f.dType),
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
			id:      a.newID(),
			pos:     a.pos(),
			Type:    "function-declaration",
			Name:    name,
			RetType: a.assignIdToDataType(retType),
			Block: &block{
				id:    a.newID(),
				Stmts: make([]stmt, 0),
			},
		}
		for _, f := range flattenFieldList(x.Type.Params) {
			param := varDecl{
				id:       a.newID(),
				pos:      a.pos(),
				Type:     "variable-declaration",
				Name:     f.vName,
				DataType: a.assignIdToDataType(f.dType),
			}
			fn.Params = append(fn.Params, param)
		}
		a.addStmt(fn)
		a.pushStmts(&fn.Block.Stmts)
	case *ast.DeclStmt:
		gd, _ := x.Decl.(*ast.GenDecl)
		for _, spec := range gd.Specs {
			s, e := spec.(*ast.ValueSpec)
			if !e {
				continue
			}
			for i, t := range flattenNames(s.Type, s.Names) {
				vType := t.dType.Name
				v, _ := zeroValues[vType]
				if s.Values != nil {
					v = exprString(s.Values[i])
				}
				decl := &varDecl{
					id:       a.newID(),
					pos:      a.pos(),
					Type:     "variable-declaration",
					Name:     t.vName,
					DataType: a.assignIdToDataType(t.dType),
					Init: &identifier{
						id:    a.newID(),
						pos:   a.pos(),
						Type:  vType,
						Value: v,
					},
				}
				a.addStmt(decl)
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
					pos:   a.pos(),
					Type:  t,
					Value: v,
				},
			}
			a.addStmt(asg)
		}
		return nil
	case *ast.File:
	case *ast.BlockStmt:
	case *ast.ExprStmt:
	case *ast.GenDecl:
	default:
		return nil
	}
	a.pushNode(node)
	return a
}
