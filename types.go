package superast

type id struct {
	ID int `json:"id"`
}

type block struct {
	id
	Stmts []stmt `json:"statements"`
}

type dataType struct {
	id
	Name string `json:"name"`
}

type varDecl struct {
	id
	Name     string    `json:"name"`
	DataType *dataType `json:"data-type,omitempty"`
}

type stmt interface{}

type statement struct {
	id
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
	id
	Line     int    `json:"line"`
	Type     string `json:"type"`
	Value    string `json:"value"`
}

type structDecl struct {
	id
	Line  int       `json:"line"`
	Type  string    `json:"type"`
	Name  string    `json:"name"`
	Attrs []varDecl `json:"attributes"`
}

