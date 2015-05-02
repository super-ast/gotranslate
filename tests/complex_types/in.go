package main

import "fmt"

type foo struct {
	a string
	b int
}

func main() {
	var f1 foo
	f2 := foo{
		a: "test",
		b: 3,
	}
	//var s1 []int
	//s2 := []string{"test1", "test2"}
	//fmt.Println(f1, f2, s1, s2)
}
