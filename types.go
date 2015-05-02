package superast

type id struct {
	ID int `json:"id"`
}

type line struct {
	Line int `json:"line"`
}

type block struct {
	id
	Stmts []stmt `json:"statements"`
}

type stmt interface{}
type value interface{}

type dataType struct {
	id
	Name string `json:"name"`
}

type varDecl struct {
	id
	line
	Type     string      `json:"type"`
	Name     string      `json:"name"`
	DataType *dataType   `json:"data-type"`
	Init     *identifier `json:"init,omitempty"`
}

type funcDecl struct {
	id
	line
	Type    string    `json:"type"`
	Name    string    `json:"name"`
	Params  []varDecl `json:"parameters,omitempty"`
	RetType *dataType `json:"return-type"`
	Block   *block    `json:"block"`
}

type identifier struct {
	id
	line
	Type  string `json:"type"`
	Value value  `json:"value"`
}

type funcCall struct {
	id
	line
	Type string `json:"type"`
	Name string `json:"name"`
	Args []stmt `json:"arguments"`
}

type structDecl struct {
	id
	line
	Type  string    `json:"type"`
	Name  string    `json:"name"`
	Attrs []varDecl `json:"attributes"`
}
