/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package gotranslate

type id struct {
	ID int `json:"id"`
}

type pos struct {
	Line int `json:"line"`
	Col  int `json:"column"`
}

type block struct {
	id
	Stmts []stmt `json:"statements"`
}

type stmt interface{}
type expr interface{}
type value interface{}

type retStmt struct {
	id
	pos
	Type string `json:"type"`
	Expr expr   `json:"expression"`
}

type conditional struct {
	id
	pos
	Type string `json:"type"`
	Cond expr   `json:"condition,omitempty"`
	Then *block `json:"then,omitempty"`
	Else *block `json:"else,omitempty"`
}

type forStmt struct {
	id
	pos
	Type  string `json:"type"`
	Init  expr   `json:"init,omitempty"`
	Cond  expr   `json:"condition,omitempty"`
	Post  expr   `json:"post,omitempty"`
	Block *block `json:"block"`
}

type dataType struct {
	id
	Name    string    `json:"name"`
	SubType *dataType `json:"data-type,omitempty"`
}

type varDecl struct {
	id
	pos
	Type     string    `json:"type"`
	Name     string    `json:"name"`
	DataType *dataType `json:"data-type"`
	Init     expr      `json:"init,omitempty"`
}

type funcDecl struct {
	id
	pos
	Type    string    `json:"type"`
	Name    string    `json:"name"`
	Params  []varDecl `json:"parameters"`
	RetType *dataType `json:"return-type"`
	Block   *block    `json:"block"`
}

type binary struct {
	id
	pos
	Type  string `json:"type"`
	Left  expr   `json:"left"`
	Right expr   `json:"right"`
}

type identifier struct {
	id
	pos
	Type  string `json:"type"`
	Value value  `json:"value"`
}

type unary struct {
	id
	pos
	Type string `json:"type"`
	Expr expr   `json:"expression"`
}

type funcCall struct {
	id
	pos
	Type string `json:"type"`
	Name string `json:"name"`
	Args []expr `json:"arguments"`
}

type structDecl struct {
	id
	pos
	Type  string    `json:"type"`
	Name  string    `json:"name"`
	Attrs []varDecl `json:"attributes"`
}

type errorNode struct {
	id
	pos
	Type  string `json:"type"`
	Value string `json:"value"`
	Desc  string `json:"description"`
}
