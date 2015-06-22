package superast

import (
	"go/ast"
	"go/token"
	//"log"
	"strconv"
	//"strings"
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
	blockStack []*block
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
	a.pushBlock(a.RootBlock)
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

func (a *AST) pushBlock(b *block) {
	a.blockStack = append(a.blockStack, b)
}

func (a *AST) curBlock() *block {
	return a.blockStack[len(a.blockStack)-1]
}

func (a *AST) addStmt(s stmt) {
	b := a.curBlock()
	b.Stmts = append(b.Stmts, s)
}

func (a *AST) getInvalid(n ast.Node, desc string) expr {
	return &errorNode{
		id:    a.newID(),
		pos:   a.nodePos(n),
		Type:  "error",
		Value: "Unsupported statement or expression",
		Desc:  desc,
	}
}

func (a *AST) addInvalid(n ast.Node, desc string) {
	a.addStmt(a.getInvalid(n, desc))
}

func (a *AST) popBlock() {
	a.blockStack = a.blockStack[:len(a.blockStack)-1]
}

func exprString(x ast.Expr) string {
	switch t := x.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.BasicLit:
		return t.Value
	case *ast.StarExpr:
		return exprString(t.X)
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
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
	switch t := x.(type) {
	case *ast.ArrayType:
		return &dataType{
			Name:    "vector",
			SubType: exprType(t.Elt),
		}
	case *ast.BasicLit:
		n, _ := basicLitName[t.Kind]
		return &dataType{
			Name: n,
		}
	case *ast.BinaryExpr:
		return exprType(t.X)
	case *ast.CompositeLit:
		n := exprString(t.Type)
		return &dataType{
			Name: n,
		}
	}
	if n := exprString(x); n != "" {
		return &dataType{
			Name: n,
		}
	}
	return &dataType{
		Name: "unknown",
	}
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
	"int":    new(int),
	"double": new(float64),
	"char":   new(rune),
	"string": new(string),
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
		var t string
		switch x.Op {
		case token.ADD:
			t = "pos"
		case token.SUB:
			t = "neg"
		case token.NOT:
			t = "not"
		//case token.XOR:
		//case token.AND:
		default:
			return a.getInvalid(expr, "Invalid unary expression: " + x.Op.String())
		}
		return &unary{
			id:   a.newID(),
			pos:  a.nodePos(x),
			Type: t,
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
		t := x.Op.String()
		switch x.Op {
		case token.LAND:
			t = "and"
		case token.LOR:
			t = "or"
		}
		return &binary{
			id:    a.newID(),
			pos:   a.nodePos(x),
			Type:  t,
			Left:  a.parseExpr(x.X),
			Right: a.parseExpr(x.Y),
		}
	case *ast.IndexExpr:
		return &binary{
			id:    a.newID(),
			pos:   a.nodePos(x),
			Type:  "[]",
			Left:  a.parseExpr(x.X),
			Right: a.parseExpr(x.Index),
		}
	case *ast.SelectorExpr:
		return &binary{
			id:    a.newID(),
			pos:   a.nodePos(x),
			Type:  ".",
			Left:  a.parseExpr(x.X),
			Right: a.parseExpr(x.Sel),
		}
	case *ast.ParenExpr:
		return a.parseExpr(x.X)
	default:
		//log.Printf("Unknown expression: %#v", x)
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
			a.popBlock()
		}
		a.popNode()
		return nil
	}
	a.pos = node.Pos()
	//log.Printf("%s%#v", strings.Repeat("  ", len(a.nodeStack)), node)
	switch x := node.(type) {
	case *ast.TypeSpec:
		n := ""
		if x.Name != nil {
			n = exprString(x.Name)
		}
		switch t := x.Type.(type) {
		case *ast.StructType:
			d := &structDecl{
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
				d.Attrs = append(d.Attrs, attr)
			}
			a.addStmt(d)
		}
		return nil
	case *ast.BasicLit:
		l := a.parseExpr(x)
		a.addStmt(l)
		return nil
	case *ast.UnaryExpr:
		u := a.parseExpr(x)
		a.addStmt(u)
		return nil
	case *ast.CallExpr:
		c := a.parseExpr(x)
		a.addStmt(c)
		return nil
	case *ast.FuncDecl:
		name := x.Name.Name
		retType := &dataType{
			Name: "void",
		}
		results := flattenFieldList(x.Type.Results)
		switch len(results) {
		case 1:
			retType = results[0].dType
		}
		d := &funcDecl{
			id:      a.newID(),
			pos:     a.nodePos(x),
			Type:    "function-declaration",
			Name:    name,
			RetType: a.assignIdToDataType(retType),
			Block: &block{
				id:    a.newID(),
				Stmts: make([]stmt, 0),
			},
			Params: make([]varDecl, 0),
		}
		for _, f := range flattenFieldList(x.Type.Params) {
			param := varDecl{
				id:       a.newID(),
				pos:      a.nodePos(f.node),
				Type:     "variable-declaration",
				Name:     f.vName,
				DataType: a.assignIdToDataType(f.dType),
			}
			d.Params = append(d.Params, param)
		}
		a.addStmt(d)
		a.pushBlock(d.Block)
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
				d := &varDecl{
					id:       a.newID(),
					pos:      a.curPos(),
					Type:     "variable-declaration",
					Name:     t.vName,
					DataType: a.assignIdToDataType(t.dType),
				}
				if v != nil {
					d.Init = &identifier{
						id:    a.newID(),
						pos:   a.curPos(),
						Type:  vType,
						Value: v,
					}
				}
				a.addStmt(d)
			}
		}
		return nil
	case *ast.AssignStmt:
		for i, l := range x.Lhs {
			r := x.Rhs[i]
			var s stmt
			if x.Tok == token.DEFINE {
				s = &varDecl{
					id:   a.newID(),
					pos:  a.curPos(),
					Type: "variable-declaration",
					Name: exprString(l),
					DataType: a.assignIdToDataType(exprType(r)),
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
	case *ast.ReturnStmt:
		r := &retStmt{
			id:   a.newID(),
			pos:  a.nodePos(x),
			Type: "return",
		}
		if len(x.Results) > 0 {
			r.Expr = a.parseExpr(x.Results[0])
		}
		a.addStmt(r)
		return nil
	case *ast.IncDecStmt:
		u := &unary{
			id:   a.newID(),
			pos:  a.nodePos(x),
			Expr: a.parseExpr(x.X),
		}
		switch x.Tok {
		case token.INC:
			u.Type = "_++"
		case token.DEC:
			u.Type = "_--"
		}
		a.addStmt(u)
		return nil
	case *ast.IfStmt:
		c := &conditional{
			id:   a.newID(),
			pos:  a.nodePos(x),
			Type: "conditional",
			Cond: a.parseExpr(x.Cond),
			Then: &block{
				id:    a.newID(),
				Stmts: make([]stmt, 0),
			},
		}
		a.addStmt(c)
		if x.Else != nil {
			c.Else = &block{
				id:    a.newID(),
				Stmts: make([]stmt, 0),
			}
			a.pushBlock(c.Else)
			a.pushNode(node)
		}
		a.pushBlock(c.Then)
	case *ast.ForStmt:
		f := &forStmt{
			id:   a.newID(),
			pos:  a.nodePos(x),
			Type: "while",
			Cond: a.parseExpr(x.Cond),
			Block: &block{
				id:    a.newID(),
				Stmts: make([]stmt, 0),
			},
		}
		if x.Init != nil {
			f.Type = "for"
			//log.Println("%T", x.Init)
			//f.Init = a.parseExpr(x.Init)
		}
		if x.Post != nil {
			f.Type = "for"
			//log.Println("%T", x.Post)
			//f.Post = a.parseExpr(x.Post)
		}
		a.addStmt(f)
		a.pushBlock(f.Block)
	case *ast.File:
	case *ast.BlockStmt:
	case *ast.ExprStmt:
	case *ast.GenDecl:
	case *ast.BranchStmt:
		a.addInvalid(node, "branch statements not supported")
		return nil
	case *ast.RangeStmt:
		a.addInvalid(node, "range statements not supported")
		return nil
	case *ast.SendStmt:
		a.addInvalid(node, "send statements not supported")
		return nil
	case *ast.SwitchStmt:
		a.addInvalid(node, "switch statements not supported")
		return nil
	case *ast.DeferStmt:
		a.addInvalid(node, "defer statements not supported")
		return nil
	case *ast.GoStmt:
		a.addInvalid(node, "go statements not supported")
		return nil
	default:
		//log.Printf("%s!%#v", strings.Repeat("  ", len(a.nodeStack)), node)
		return nil
	}
	a.pushNode(node)
	return a
}
