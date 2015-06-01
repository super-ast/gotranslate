package main

import "fmt"

func main() {
	i := 3
	i = -i
	i = +i
	i = -+-i
	fmt.Println(i)
}
