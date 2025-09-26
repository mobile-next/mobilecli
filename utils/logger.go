package utils

import (
	"log"
)

var (
	isVerbose bool
)

func SetVerbose(verbose bool) {
	isVerbose = verbose
}

func IsVerbose() bool {
	return isVerbose
}

func Verbose(format string, args ...interface{}) {
	if isVerbose {
		log.Printf("[VERBOSE] "+format, args...)
	}
}

func Info(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}
