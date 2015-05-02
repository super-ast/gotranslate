package superast

import (
	"encoding/json"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

var (
	write = flag.Bool("write", false, "Write json results")
	name  = flag.String("name", "", "Test name")
)

func init() {
	flag.Parse()
}

func toJSON(t *testing.T, a *AST) []byte {
	b, err := json.MarshalIndent(a.RootBlock, "", "  ")
	if err != nil {
		t.Errorf("Could not generate JSON from AST: %s", err)
	}
	b = append(b, '\n')
	return b
}

const (
	testsDir = "tests"
	inFilename = "in.go"
	outFilename = "out.json"
)

func doTest(t *testing.T, name string) {
	fset := token.NewFileSet()
	in, err := os.Open(path.Join(testsDir, name, inFilename))
	if err != nil {
		t.Errorf("Failed opening file: %s", err)
	}
	f, err := parser.ParseFile(fset, name+".go", in, 0)
	if err != nil {
		t.Errorf("Failed parsing source file: %s", err)
	}
	a := NewAST(fset)
	ast.Walk(a, f)
	got := toJSON(t, a)
	outPath := path.Join(testsDir, name, outFilename)
	if *write {
		out, err := os.Create(outPath)
		if err != nil {
			t.Errorf("Failed opening file: %s", err)
		}
		_, err = out.Write(got)
		if err != nil {
			t.Errorf("Failed writing json file: %s", err)
		}
	} else {
		out, err := os.Open(outPath)
		if err != nil {
			t.Errorf("Failed opening file: %s", err)
		}
		want, err := ioutil.ReadAll(out)
		if err != nil {
			t.Errorf("Failed reading json file: %s", err)
		}
		if string(want) != string(got) {
			t.Errorf("Mismatching JSON outputs in the test '%s'", name)
		}
	}
}

func TestCases(t *testing.T) {
	entries, err := ioutil.ReadDir(testsDir)
	if err != nil {
		return
	}
	if *name != "" {
		doTest(t, *name)
	} else {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			doTest(t, e.Name())
		}
	}
}
