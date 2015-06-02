package main

import "fmt"
import "go/token"

func main() {
	var a []int
	fmt.Println(a)
	fmt.Println(a[0])
	fmt.Println(token.COMMENT)
}
