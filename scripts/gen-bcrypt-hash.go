// gen-bcrypt-hash.go — one-off helper to generate a bcrypt hash for the demo
// password. Run with: `go run scripts/gen-bcrypt-hash.go`
//
// Usage:
//   go run scripts/gen-bcrypt-hash.go              # uses default "demo123" / cost=10
//   go run scripts/gen-bcrypt-hash.go mySecret 12  # custom password + cost
package main

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "demo123"
	cost := 10
	if len(os.Args) > 1 {
		password = os.Args[1]
	}
	if len(os.Args) > 2 {
		c, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid cost:", err)
			os.Exit(1)
		}
		cost = c
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bcrypt:", err)
		os.Exit(1)
	}
	fmt.Println(string(h))
}
