// +build !debug

package debug

// Println will be optimized away.
func Println(a ...interface{}) {}

// Printf will be optimized away.
func Printf(format string, a ...interface{}) {}
