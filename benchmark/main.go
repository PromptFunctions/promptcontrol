//go:build !compliance && !large
// +build !compliance,!large

package main

import "fmt"

func main() {
	fmt.Println("Select a benchmark tag: -tags compliance or -tags large")
}
