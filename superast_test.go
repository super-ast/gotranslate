package superast

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func toJSON(t *testing.T, a *AST) []byte {
	b, err := json.MarshalIndent(a.RootBlock, "", "  ")
	if err != nil {
		t.Errorf("Could not generate JSON from AST: %s", err)
	}
	b = append(b, '\n')
	return b
}

func doTest(t *testing.T, name string, in, out io.Reader) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name+".go", in, 0)
	if err != nil {
		t.Errorf("Failed parsing source file: %s", err)
	}
	a := NewAST(fset)
	ast.Walk(a, f)
}

const testsDir = "tests"

func TestCases(t *testing.T) {
	entries, err := ioutil.ReadDir(testsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		in, err := os.Open(path.Join(testsDir, name, name+".go"))
		if err != nil {
			t.Errorf("Failed opening file: %s", err)
		}
		out, err := os.Open(path.Join(testsDir, name, name+".json"))
		if err != nil {
			t.Errorf("Failed opening file: %s", err)
		}
		doTest(t, name, in, out)
	}
}
