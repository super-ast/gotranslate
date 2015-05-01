package main

import "fmt"

func main() {
	var a int
	var b string
	a, b = 3, "foo"
	c, d := 5, 6
	fmt.Println(a, b, c, d)
}
