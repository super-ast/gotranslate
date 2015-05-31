package superast

import (
	"go/ast"
	"go/token"
	"log"
	"strconv"
	"strings"
)

const (
	Default = iota
	IfBody
	IfElse
	FuncBody
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

func (a *AST) nodePos(n ast.Node) pos {
	return a.newPos(n.Pos())
}

func (a *AST) curPos() pos {
	return a.newPos(a.pos)
}

func (a *AST) pushNode(node ast.Node) {
	a.nodeStack = append(a.nodeStack, node)
}

func (a *AST) curNode() ast.Node {
	return a.nodeStack[len(a.nodeStack)-1]
}

func (a *AST) popNode() {
	a.nodeStack = a.nodeStack[:len(a.nodeStack)-1]
}

func (a *AST) pushStmts(stmts *[]stmt) {
	a.stmtsStack = append(a.stmtsStack, stmts)
}

func (a *AST) curStmts() *[]stmt {
	return a.stmtsStack[len(a.stmtsStack)-1]
}

func (a *AST) addStmt(s stmt) {
	curStmts := a.curStmts()
	*curStmts = append(*curStmts, s)
}

func (a *AST) popStmts() {
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
	node  ast.Node
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
			node:  n,
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
	"char":   '\x00',
	"string": "",
}

func (a *AST) parseExpr(expr ast.Expr) expr {
	switch x := expr.(type) {
	case *ast.Ident:
		return &identifier{
			id:    a.newID(),
			pos:   a.nodePos(x),
			Type:  "identifier",
			Value: x.Name,
		}
	case *ast.BasicLit:
		lType, _ := basicLitName[x.Kind]
		return &identifier{
			id:    a.newID(),
			pos:   a.nodePos(x),
			Type:  lType,
			Value: exprValue(x),
		}
	case *ast.UnaryExpr:
		return &unary{
			id:   a.newID(),
			pos:  a.nodePos(x),
			Type: x.Op.String(),
			Expr: a.parseExpr(x.X),
		}
	case *ast.CallExpr:
		call := &funcCall{
			id:   a.newID(),
			pos:  a.nodePos(x),
			Type: "function-call",
			Name: exprString(x.Fun),
		}
		for _, e := range x.Args {
			call.Args = append(call.Args, a.parseExpr(e))
		}
		return call
	case *ast.BinaryExpr:
		return &binary{
			id:    a.newID(),
			pos:   a.nodePos(x),
			Type:  x.Op.String(),
			Left:  a.parseExpr(x.X),
			Right: a.parseExpr(x.Y),
		}
	case *ast.ParenExpr:
		return a.parseExpr(x.X)
	default:
		log.Printf("Unknown expression: %#v", x)
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
	if node == nil {
		switch a.curNode().(type) {
		case *ast.BlockStmt:
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
		return nil
	case *ast.BasicLit:
		lit := a.parseExpr(x)
		a.addStmt(lit)
		return nil
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
			pos:     a.nodePos(x),
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
				pos:      a.nodePos(f.node),
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
		return nil
	case *ast.AssignStmt:
		for i, l := range x.Lhs {
			r := x.Rhs[i]
			var t string
			switch rx := r.(type) {
			case *ast.BasicLit:
				t, _ = basicLitName[rx.Kind]
			case *ast.CompositeLit:
				t = exprString(rx.Type)
			}
			var s stmt
			if x.Tok == token.DEFINE {
				s = &varDecl{
					id:   a.newID(),
					pos:  a.curPos(),
					Type: "variable-declaration",
					Name: exprString(l),
					DataType: &dataType{
						id:   a.newID(),
						Name: t,
					},
					Init: a.parseExpr(r),
				}
			} else {
				s = &binary{
					id:    a.newID(),
					pos:   a.curPos(),
					Type:  x.Tok.String(),
					Left:  a.parseExpr(l),
					Right: a.parseExpr(r),
				}
			}
			a.addStmt(s)
		}
		return nil
	case *ast.IfStmt:
		cond := &conditional{
			id:   a.newID(),
			pos:  a.nodePos(x),
			Type: "conditional",
			Cond: a.parseExpr(x.Cond),
			Then: &block{
				id:    a.newID(),
				Stmts: make([]stmt, 0),
			},
		}
		a.addStmt(cond)
		if x.Else != nil {
			cond.Else = &block{
				id:    a.newID(),
				Stmts: make([]stmt, 0),
			}
			a.pushStmts(&cond.Else.Stmts)
			a.pushNode(node)
		}
		a.pushStmts(&cond.Then.Stmts)
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
