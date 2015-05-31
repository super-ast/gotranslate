package main

import "fmt"

func main() {
	if 3 < 4 {
		fmt.Println("foo")
	} else if 4 < 5 {
		fmt.Println("bar")
	} else {
		fmt.Println("nil")
	}
}
