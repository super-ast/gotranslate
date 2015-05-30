package superast

import (
	"go/ast"
	"go/token"
	"log"
	"strconv"
	"strings"
)

type AST struct {
	curID      int
	RootBlock  *block
	nodeStack  []ast.Node
	stmtsStack []*[]stmt
	fset       *token.FileSet
	pos        token.Pos
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

func (a *AST) newPos(p token.Pos) pos {
	position := a.fset.Position(p)
	return pos{Line: position.Line, Col: position.Column}
}

func (a *AST) curPos() pos {
	return a.newPos(a.pos)
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

func exprValue(x ast.Expr) value {
	switch t := x.(type) {
	case *ast.BasicLit:
		switch t.Kind {
		case token.INT:
			i, _ := strconv.ParseInt(t.Value, 10, 0)
			return i
		case token.FLOAT:
			f, _ := strconv.ParseFloat(t.Value, 64)
			return f
		case token.CHAR:
			r, _, _, _ := strconv.UnquoteChar(t.Value, '\'')
			return r
		case token.STRING:
			s, _ := strconv.Unquote(t.Value)
			return s
		}
		return t.Value
	}
	if v := exprString(x); v != "" {
		return v
	}
	return nil
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
			{vName: "", dType: t},
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

func (a *AST) parseExpr(expr ast.Expr) expr {
	switch x := expr.(type) {
	case *ast.Ident:
		return &identifier{
			id:    a.newID(),
			pos:   a.newPos(x.NamePos),
			Type:  "identifier",
			Value: x.Name,
		}
	case *ast.BasicLit:
		lType, _ := basicLitName[x.Kind]
		return &identifier{
			id:    a.newID(),
			pos:   a.newPos(x.ValuePos),
			Type:  lType,
			Value: exprValue(x),
		}
	case *ast.UnaryExpr:
		return &unary{
			id:   a.newID(),
			pos:  a.newPos(x.OpPos),
			Type: x.Op.String(),
			Expr: a.parseExpr(x.X),
		}
	case *ast.CallExpr:
		call := &funcCall{
			id:   a.newID(),
			pos:  a.curPos(),
			Type: "function-call",
			Name: exprString(x.Fun),
		}
		for _, e := range x.Args {
			call.Args = append(call.Args, a.parseExpr(e))
		}
		return call
	}
	return nil
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
	a.pos = node.Pos()
	log.Printf("%s%#v", strings.Repeat("  ", len(a.nodeStack)), node)
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
				pos:  a.curPos(),
				Type: "struct-declaration",
				Name: n,
			}
			for _, f := range flattenFieldList(t.Fields) {
				attr := varDecl{
					id:       a.newID(),
					pos:      a.curPos(),
					Type:     "variable-declaration",
					Name:     f.vName,
					DataType: a.assignIdToDataType(f.dType),
				}
				decl.Attrs = append(decl.Attrs, attr)
			}
			a.addStmt(decl)
		}
	case *ast.BasicLit:
		lit := a.parseExpr(x)
		a.addStmt(lit)
	case *ast.UnaryExpr:
		unary := a.parseExpr(x)
		a.addStmt(unary)
		return nil
	case *ast.CallExpr:
		call := a.parseExpr(x)
		a.addStmt(call)
		return nil
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
			pos:     a.curPos(),
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
				pos:      a.curPos(),
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
			s, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, t := range flattenNames(s.Type, s.Names) {
				vType := t.dType.Name
				v, _ := zeroValues[vType]
				if s.Values != nil {
					v = exprValue(s.Values[i])
				}
				decl := &varDecl{
					id:       a.newID(),
					pos:      a.curPos(),
					Type:     "variable-declaration",
					Name:     t.vName,
					DataType: a.assignIdToDataType(t.dType),
					Init: &identifier{
						id:    a.newID(),
						pos:   a.curPos(),
						Type:  vType,
						Value: v,
					},
				}
				a.addStmt(decl)
			}
		}
	case *ast.AssignStmt:
		aType := x.Tok.String()
		switch x.Tok {
		case token.DEFINE:
			aType = "variable-declaration"
		}
		for i, l := range x.Lhs {
			var t string
			r := x.Rhs[i]
			switch rx := r.(type) {
			case *ast.BasicLit:
				t, _ = basicLitName[rx.Kind]
			case *ast.CompositeLit:
				t = exprString(rx.Type)
			default:
			}
			asg := &varDecl{
				id:   a.newID(),
				pos:  a.curPos(),
				Type: aType,
				Name: exprString(l),
				DataType: &dataType{
					id:   a.newID(),
					Name: t,
				},
				Init: a.parseExpr(r),
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
