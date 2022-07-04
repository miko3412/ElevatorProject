package types

import (
	"fmt"
)

type MotorDirection byte

const (
	MotorUp   MotorDirection = 1
	MotorDown                = 255
	MotorHalt                = 0
)

type ElevatorState int

const (
	Standby ElevatorState = iota // Zero orders, in floor with door closed.
	Carring                      // Door open in floor, waiting for timer.
	Moving                       // Moving. Guaranteed one or more orders.
)

type Call int
type Floor = int

const (
	HallUp Call = iota
	HallDown
	Car
)

type Order struct {
	C Call
	F Floor
}

func (c Call) String() string {
	switch c {
	case HallUp:
		return "HallUp"
	case Car:
		return "Car"
	case HallDown:
		return "HallDown"
	}
	return "Unknown"
}

func (o Order) String() string {
	return fmt.Sprintf("%s on floor %d", o.C, o.F)
}
