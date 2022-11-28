package util

import (
	"log"
)

const Debug = false
const Specific = true

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

func SpecificPrintf(format string, a ...interface{}) (n int, err error) {
	if Specific {
		log.Printf(format, a...)
	}
	return
}
