package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mvdan/superast"
)

var (
	pretty = flag.Bool("p", false, "indent (pretty print) output")
)

func main() {
	flag.Parse()
	src := `
package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	ast := superast.ParseString(src)

	if *pretty {
		b, err := json.Marshal(ast.RootBlock)
		if err != nil {
			log.Fatal(err)
		}
		var out bytes.Buffer
		if err := json.Indent(&out, b, "", "  "); err != nil {
			log.Fatal(err)
		}
		if _, err := out.WriteTo(os.Stdout); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\n")
	} else {
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(ast.RootBlock); err != nil {
			log.Println(err)
		}
	}

}
