package main

import "fmt"

func main() {
	goto foo
	i := 3
	foo:
	fmt.Println(i)
}
