package probe

func Bool2int(b bool) int {
	// the compiler currently optimizes only this form (see issue 6011)
	var i int
	if b {
		i = 1
	} else {
		i = 0
	}
	return i
}
