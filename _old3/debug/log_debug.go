// +build debug

package debug

import (
	"fmt"
)

// Println will be optimized away.
func Println(a ...interface{}) {
	fmt.Println(a...)
}

// Printf will be optimized away.
func Printf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}
