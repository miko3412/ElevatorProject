package network

import (
	"fmt"
	"math"
	"project-group-81/config"
	"project-group-81/elevator"
	"project-group-81/types"
)

const STOPPING_COST = 30
const DISTANCE_COST = 25

func getHighestOrder(elevator elevator.Elevator) types.Floor {
	highest := 0
	for order := range elevator.Orders {
		if highest < order.F {
			highest = order.F
		}
	}
	return highest
}

func getLowestOrder(elevator elevator.Elevator) types.Floor {
	lowest := config.NUMBER_OF_FLOORS
	for order := range elevator.Orders {
		if lowest > order.F {
			lowest = order.F
		}
	}
	return lowest
}

// Simple simulation of elevator running. An easy implementation method, but not the most efficient.
func orderBetween(elevator elevator.Elevator, order, destination types.Order) bool {
	highestFloor := getHighestOrder(elevator)
	lowestFloor := getLowestOrder(elevator)
	currentFloor := elevator.LastFloor
	var simulatorDirection types.MotorDirection
	if elevator.State == types.Standby {
		if elevator.LastFloor > order.F {
			simulatorDirection = types.MotorDown
		} else if elevator.LastFloor < order.F {
			simulatorDirection = types.MotorUp
		} else {
			return false
		}
	} else {
		simulatorDirection = elevator.LastDirection
	}
	for i := 0; i < 12; i++ { // This is a workaround for a bug we didn't have time to fix.
		var servableCall types.Call
		switch simulatorDirection {
		case types.MotorUp:
			servableCall = types.HallUp
		case types.MotorDown:
			servableCall = types.HallDown
		case types.MotorHalt:
			fmt.Printf("Warning: Standby elevator should never call orderBetween")
		}

		if (order == types.Order{C: servableCall, F: currentFloor}) {
			return true
		} else if (destination == types.Order{C: servableCall, F: currentFloor} ||
			destination == types.Order{C: types.Car, F: currentFloor}) {
			return false
		}

		switch simulatorDirection {
		case types.MotorUp:
			currentFloor++
			if currentFloor > highestFloor {
				currentFloor--
				simulatorDirection = types.MotorDown
			}
		case types.MotorDown:
			currentFloor--
			if currentFloor < lowestFloor {
				currentFloor++
				simulatorDirection = types.MotorUp
			}
		case types.MotorHalt:
			fmt.Printf("Warning: types.Standby elevator should never call orderBetween")
		}
	}
	return false
}

func stopsBetween(destination types.Order, elevator elevator.Elevator) int {
	stops := 0
	for order := range elevator.Orders {
		if orderBetween(elevator, order, destination) {
			stops++
		}
	}
	return stops
}

func distance(order types.Order, elevator elevator.Elevator) int {
	if elevator.State == types.Standby {
		if order.F > elevator.LastFloor {
			return order.F - elevator.LastFloor
		} else {
			return elevator.LastFloor - order.F
		}
	}
	switch elevator.LastDirection {
	case types.MotorUp:
		switch order.C {
		case types.HallUp:
			if elevator.LastFloor < order.F {
				return order.F - elevator.LastFloor
			} else {
				return 2*(getHighestOrder(elevator)-getLowestOrder(elevator)) - elevator.LastFloor + order.F
			}
		case types.HallDown:
			return 2*getHighestOrder(elevator) - elevator.LastFloor - order.F
		}
	case types.MotorDown:
		switch order.C {
		case types.HallUp:
			return order.F + elevator.LastFloor - 2*getLowestOrder(elevator)
		case types.HallDown:
			if elevator.LastFloor > order.F {
				return elevator.LastFloor - order.F
			} else {
				return 2*(getHighestOrder(elevator)-getLowestOrder(elevator)) - order.F + elevator.LastFloor
			}
		}
	}
	return 2 * config.NUMBER_OF_FLOORS // Never reached
}

func cost(order types.Order, elevator elevator.Elevator) int {
	distanceCost := distance(order, elevator) * DISTANCE_COST
	stopCost := stopsBetween(order, elevator) * STOPPING_COST
	if elevator.Orders.Contains(types.Order{C: types.Car, F: order.F}) { // Discount if elevator is already stopping there
		return distanceCost + stopCost - STOPPING_COST
	} else {
		return distanceCost + stopCost
	}
}

func Assign(order types.Order, processes []Process) AssignedOrder {
	var cheapestProcess Process
	minCost := math.MaxInt32
	for _, p := range processes {
		if p.Active {
			cost := cost(order, p.Elevator)
			if minCost > cost {
				minCost = cost
				cheapestProcess = p
			}
		}
	}
	return AssignedOrder{cheapestProcess.Id, order}
}
