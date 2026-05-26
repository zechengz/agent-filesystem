package main

import "fmt"

type LiveSkillsError struct {
	Message string
	Code    int
}

func (e *LiveSkillsError) Error() string {
	return e.Message
}

func fail(message string, args ...any) *LiveSkillsError {
	return &LiveSkillsError{Message: fmt.Sprintf(message, args...), Code: 1}
}

func require(condition bool, message string, args ...any) error {
	if !condition {
		return fail(message, args...)
	}
	return nil
}
