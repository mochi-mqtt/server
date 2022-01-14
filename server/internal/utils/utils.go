package utils

// InSliceString returns true if a string exists in a slice of strings.
// This temporary and should be replaced with a function from the new
// go slices package in 1.19 when available.
// https://github.com/golang/go/issues/45955
func InSliceString(sl []string, st string) bool {
	for _, v := range sl {
		if st == v {
			return true
		}
	}
	return false
}
