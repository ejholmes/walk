package main

import "fmt"

func ansi(code string, format string, v ...interface{}) string {
	if !isTTY {
		return fmt.Sprintf(format, v...)
	}
	return fmt.Sprintf(fmt.Sprintf("\x1b[%sm%s\x1b[0m", code, format), v...)
}
