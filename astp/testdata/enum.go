package testdata

type Status int

const (
	StatusActive Status = iota
	StatusInactive
	StatusPending
)

type Priority int

const (
	PriorityLow    Priority = 1
	PriorityMedium Priority = 2
	PriorityHigh   Priority = 3
)
