package elevator

import (
	"fmt"
	"project-group-81/config"
	"project-group-81/hardware"
	"project-group-81/types"
	"time"
)

type Elevator struct {
	LastFloor     types.Floor
	State         types.ElevatorState
	LastDirection types.MotorDirection
	Orders        types.OrderSet
}

func DefaultElevator() Elevator {
	return Elevator{types.Floor(0), types.Standby, types.MotorHalt, make(map[types.Order]bool)}
}

func (e *Elevator) initPhase(hc *hardware.HardwareConn) {
	for i := 0; i < config.NUMBER_OF_FLOORS; i++ {
		for j := 0; j < 3; j++ {
			hc.WriteOrderButtonLight(types.Order{C: types.Call(j), F: i}, false)
		}
	}

	hc.WriteMotorDirection(types.MotorDown)
	e.LastDirection = types.MotorDown
	for {
		if inFloor, floor := hc.ReadFloorSensor(); inFloor {
			hc.WriteMotorDirection(types.MotorHalt)
			hc.WriteFloorIndicator(floor)
			e.LastFloor = floor
			e.State = types.Standby
			break
		}
	}
}

func RunElevator(hc *hardware.HardwareConn,
	newOrderChan,
	finishedOrderChan chan<- types.Order,
	lightOnChan,
	lightOffChan <-chan types.Order,
	stateChan chan<- Elevator,
	assignedOrderChan <-chan types.Order) {

	floorChan := make(chan types.Floor)
	buttonChan := make(chan types.Order)
	doorTimer := time.NewTimer(config.DOOR_OPEN_TIME)
	inactiveTimer := time.NewTimer(config.INACTIVE_TIME)
	notificationPeriod := 3 * time.Second
	notificationTimer := time.NewTimer(notificationPeriod)

	go pollFloorSensor(hc, floorChan)
	go pollButtons(hc, buttonChan)
	go pollObstruction(hc, doorTimer)

	e := DefaultElevator()
	e.initPhase(hc)

	for {
		select {
		case floor := <-floorChan:
			if floor != e.LastFloor {
				inactiveTimer.Reset(config.INACTIVE_TIME)
				hc.WriteFloorIndicator(floor)
				e.LastFloor = floor
				if e.shouldOpen() {
					e.open(hc, doorTimer, inactiveTimer)
					inactiveTimer.Reset(config.INACTIVE_TIME)
				} else if e.shouldTurn() {
					e.turn()
					if e.shouldOpen() {
						e.open(hc, doorTimer, inactiveTimer)
					}
				}
				go func() {
					stateChan <- e
				}()
				notificationTimer.Reset(notificationPeriod)
			}
		case order := <-buttonChan:
			if order.C == types.Car {
				if !e.Orders.Contains(order) {
					e.Orders.Insert(order)
					hc.WriteOrderButtonLight(order, true)
					if e.State == types.Standby {
						if e.LastFloor == order.F {
							switch order.C {
							case types.HallDown:
								e.LastDirection = types.MotorDown
							case types.HallUp:
								e.LastDirection = types.MotorUp
							}
							e.open(hc, doorTimer, inactiveTimer)
						} else {
							e.startMoving(hc, inactiveTimer)
							e.State = types.Moving
						}
					}
					go func() {
						stateChan <- e
					}()
					notificationTimer.Reset(notificationPeriod)
				}
			} else {
				go func(order types.Order) {
					newOrderChan <- order
				}(order)
			}
		case <-doorTimer.C:
			if e.State == types.Carring {
				inactiveTimer.Reset(config.INACTIVE_TIME)
				e.removeLastFloorOrders(hc, finishedOrderChan)
				if e.Orders.IsEmpty() {
					hc.WriteDoorOpenLight(false)
					e.State = types.Standby
				} else if e.shouldTurn() {
					e.turn()
					if e.shouldOpen() {
						doorTimer.Reset(config.DOOR_OPEN_TIME)
					} else {
						hc.WriteDoorOpenLight(false)
						e.continueMoving(hc)
						e.State = types.Moving
					}
				} else {
					hc.WriteDoorOpenLight(false)
					e.continueMoving(hc)
					e.State = types.Moving
				}
				go func() {
					stateChan <- e
				}()
				notificationTimer.Reset(notificationPeriod)
			}
		case <-notificationTimer.C:
			go func() {
				stateChan <- e
			}()
			notificationTimer.Reset(notificationPeriod)
		case order := <-assignedOrderChan:
			e.Orders.Insert(order)
			hc.WriteOrderButtonLight(order, true)
			if e.State == types.Standby {
				if e.LastFloor == order.F {
					switch order.C {
					case types.HallDown:
						e.LastDirection = types.MotorDown
					case types.HallUp:
						e.LastDirection = types.MotorUp
					}
					e.open(hc, doorTimer, inactiveTimer)
					inactiveTimer.Reset(config.INACTIVE_TIME)
				} else {
					e.startMoving(hc, inactiveTimer)
					e.State = types.Moving
				}
			}
			go func() {
				stateChan <- e
			}()
			notificationTimer.Reset(notificationPeriod)
		case order := <-lightOffChan:
			hc.WriteOrderButtonLight(order, false)
		case order := <-lightOnChan:
			hc.WriteOrderButtonLight(order, true)
		case <-inactiveTimer.C:
			if e.State != types.Standby {
				fmt.Printf("Elevator inactive; stopping single elevator.\n")
				return
			}
		}
	}
}

func pollFloorSensor(hc *hardware.HardwareConn, c chan<- types.Floor) {
	for {
		if inFloor, floor := hc.ReadFloorSensor(); inFloor {
			c <- floor
		}
	}
}

func pollButtons(hc *hardware.HardwareConn, c chan<- types.Order) {
	for {
		for i := 0; i < 3; i++ {
			call := types.Call(i)
			for j := 0; j < config.NUMBER_OF_FLOORS; j++ {
				floor := types.Floor(j)
				order := types.Order{C: call, F: floor}
				pressed := hc.ReadOrderButton(order)
				if pressed {
					c <- order
				}
			}
		}
	}
}

func pollObstruction(hc *hardware.HardwareConn, doorTimer *time.Timer) {
	for {
		if hc.ReadObstructionSwitch() {
			doorTimer.Reset(config.DOOR_OPEN_TIME)
		}
	}
}

// Should never be called outside floors. But consider taking floor as an argument for explicitness
func (e *Elevator) shouldOpen() bool {
	if e.Orders.Contains(types.Order{C: types.Car, F: types.Floor(e.LastFloor)}) {
		return true
	} else {
		switch e.LastDirection {
		case types.MotorUp:
			if e.Orders.Contains(types.Order{C: types.HallUp, F: types.Floor(e.LastFloor)}) {
				return true
			}
		case types.MotorDown:
			if e.Orders.Contains(types.Order{C: types.HallDown, F: types.Floor(e.LastFloor)}) {
				return true
			}
		}
		return false
	}
}

func (e *Elevator) shouldTurn() bool {
	switch e.LastDirection {
	case types.MotorUp:
		for o := range e.Orders {
			if int(o.F) > int(e.LastFloor) {
				return false
			}
		}
	case types.MotorDown:
		for o := range e.Orders {
			if int(o.F) < int(e.LastFloor) {
				return false
			}
		}
	case types.MotorHalt:
		return false
	}
	return true
}

func (e *Elevator) turn() {
	switch e.LastDirection {
	case types.MotorUp:
		e.LastDirection = types.MotorDown
	case types.MotorDown:
		e.LastDirection = types.MotorUp
	}
}

func (e *Elevator) open(hc *hardware.HardwareConn, doorTimer *time.Timer, inactiveTimer *time.Timer) {
	hc.WriteMotorDirection(types.MotorHalt)
	hc.WriteDoorOpenLight(true)
	e.State = types.Carring
	doorTimer.Reset(config.DOOR_OPEN_TIME)
	inactiveTimer.Reset(config.INACTIVE_TIME)
}

func (e *Elevator) startMoving(hc *hardware.HardwareConn, inactiveTimer *time.Timer) {
	inactiveTimer.Reset(config.INACTIVE_TIME)
	for o := range e.Orders {
		if int(o.F) > int(e.LastFloor) {
			hc.WriteMotorDirection(types.MotorUp)
			e.LastDirection = types.MotorUp
			break
		} else if int(o.F) < int(e.LastFloor) {
			hc.WriteMotorDirection(types.MotorDown)
			e.LastDirection = types.MotorDown
			break
		}
	}
}

func (e *Elevator) removeLastFloorOrders(hc *hardware.HardwareConn, finishedOrderChan chan<- types.Order) {
	o := types.Order{C: types.Car, F: e.LastFloor}
	e.Orders.Remove(o)
	hc.WriteOrderButtonLight(o, false)
	switch e.LastDirection {
	case types.MotorDown:
		o := types.Order{C: types.HallDown, F: e.LastFloor}
		e.Orders.Remove(o)
		hc.WriteOrderButtonLight(o, false)
		go func(o types.Order) {
			finishedOrderChan <- o
		}(o)
	case types.MotorUp:
		o := types.Order{C: types.HallUp, F: e.LastFloor}
		e.Orders.Remove(o)
		hc.WriteOrderButtonLight(o, false)
		go func(o types.Order) {
			finishedOrderChan <- o
		}(o)
	}
}

func (e *Elevator) continueMoving(hc *hardware.HardwareConn) {
	hc.WriteMotorDirection(e.LastDirection)
}
