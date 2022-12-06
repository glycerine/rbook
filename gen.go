package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

// super crude, hand edited in emacs, saved to saved.go
func generate_checkmark_embedded() {
	by, _ := ioutil.ReadFile("green_checkmark.png")
	fd, _ := os.Create("checkmark.go")

	n := len(by)
	for i := 0; i < n; i += 12 {
		w := 12
		if i+w >= n {
			w = n - i
		}
		fmt.Fprintf(fd, "%#v\n", by[i:i+w])
	}
	fd.Close()
}
