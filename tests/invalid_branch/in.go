package main

import "fmt"

func main() {
	i := 3
	goto foo
	fmt.Println(i)
foo:
}
