/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package superast

import (
	"encoding/json"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

var (
	write = flag.Bool("write", false, "Write json results")
	name  = flag.String("name", "", "Test name")
)

func init() {
	flag.Parse()
}

func toJSON(t *testing.T, a *AST) ([]byte, error) {
	b, err := json.MarshalIndent(a.RootBlock, "", "  ")
	if err != nil {
		return nil, err
	}
	b = append(b, '\n')
	return b, nil
}

const (
	testsDir    = "tests"
	inFilename  = "in.go"
	outFilename = "out.json"
)

func doTest(t *testing.T, name string) {
	fset := token.NewFileSet()
	in, err := os.Open(filepath.Join(testsDir, name, inFilename))
	if err != nil {
		t.Errorf("Failed opening file: %v", err)
		return
	}
	defer in.Close()
	f, err := parser.ParseFile(fset, name+".go", in, 0)
	if err != nil {
		t.Errorf("Failed parsing source file: %v", err)
		return
	}
	a := NewAST(fset)
	ast.Walk(a, f)
	got, err := toJSON(t, a)
	if err != nil {
		t.Errorf("Could not generate JSON from AST: %v", err)
		return
	}
	outPath := filepath.Join(testsDir, name, outFilename)
	if *write {
		out, err := os.Create(outPath)
		if err != nil {
			t.Errorf("Failed opening file: %v", err)
			return
		}
		defer out.Close()
		_, err = out.Write(got)
		if err != nil {
			t.Errorf("Failed writing json file: %v", err)
			return
		}
	} else {
		out, err := os.Open(outPath)
		if err != nil {
			t.Errorf("Failed opening file: %v", err)
			return
		}
		defer out.Close()
		want, err := ioutil.ReadAll(out)
		if err != nil {
			t.Errorf("Failed reading json file: %v", err)
			return
		}
		if string(want) != string(got) {
			t.Errorf("Mismatching JSON outputs in the test '%v'", name)
			return
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
