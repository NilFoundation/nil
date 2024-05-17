package common

func Require(flag bool) {
	if !flag {
		// Maybe collect stack trace and optionally add a message from the caller.
		panic("requirement not met")
	}
}

func Check(err error) {
	if err != nil {
		panic(err)
	}
}
