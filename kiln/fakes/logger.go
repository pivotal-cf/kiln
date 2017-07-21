package fakes

import "fmt"

type Logger struct {
	PrintlnCall struct {
		Receives struct {
			LogLines []string
		}
	}
}

func (l *Logger) Println(v ...interface{}) {
	l.PrintlnCall.Receives.LogLines = append(l.PrintlnCall.Receives.LogLines, fmt.Sprintf("%s", v...))
}
