package logger

import "fmt"

type logger struct {
	enabled bool
}

var _logger logger

func Enable() {
	_logger.enabled = true
}

func Disable() {
	_logger.enabled = false
}

func Log(args ...any) {
	if _logger.enabled {
		fmt.Println(args...)
	}
}

func Logf(fstr string, args ...any) {
	if _logger.enabled {
		fmt.Printf(fstr, args...)
	}
}
