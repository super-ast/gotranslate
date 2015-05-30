package main

import "fmt"

func foo(a, b int, c string) string {
	return fmt.Sprintf("%d %d %s", a, b, c)
}

func main() {
	fmt.Println(foo(1, 2, "bar"))
}
