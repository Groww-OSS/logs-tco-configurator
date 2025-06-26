package promtail

import "fmt"

// custom errors for the promtail package
type NotASamplingStageError struct {
	msg string
}

func (e *NotASamplingStageError) Error() string {
	return e.msg
}

func NewNotSamplingStageError(msg string) error {
	return &NotASamplingStageError{msg: msg}
}

type CanNotCreateSamplingStage struct {
	msg string
}

func (e *CanNotCreateSamplingStage) Error() string {
	return "failed to create sampling stage: " + e.msg
}

func NewCanNotCreateSamplingStageError(msg string) error {
	return &CanNotCreateSamplingStage{msg: msg}
}

type OutOfRangePercentageError struct {
	percentage float64
}

func (e *OutOfRangePercentageError) Error() string {
	return fmt.Sprintf("percentage should be between 0 and 100, but provided: %f", e.percentage)
}

func NewOutOfRangePercentageError(percentage float64) error {
	return &OutOfRangePercentageError{percentage: percentage}
}
