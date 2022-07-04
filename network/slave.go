package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"project-group-81/elevator"
	"project-group-81/types"
)

func listenToMaster(
	conn net.Conn,
	assignedOrdersChan chan<- []AssignedOrder,
	slavesConsistentChan chan<- bool,
	processesChan chan<- []Process,
	masterUnreachableChan chan<- bool) {

	buf := make([]byte, MESSAGE_BUFFER_LENGTH)

	for {
		length, err := patientRead(conn, buf, MASTER_RESPONSE_TIMEOUT)
		if err != nil {
			fmt.Printf("Error receiving from master: %v\n", err)
			masterUnreachableChan <- true
			return
		} else if length == 0 {
			continue
		}

		for _, message := range decode(buf[:length]) {
			messageFlag := message[0]
			switch messageFlag {
			case ASSIGNED_ORDERS_FLAG:
				trimMessage := bytes.Trim(message[1:], "\x00")
				var assignedOrders []AssignedOrder
				err = json.Unmarshal(trimMessage, &assignedOrders)
				if err != nil {
					fmt.Printf("Error unmarshalling master message, got: %s\n", trimMessage)
				}
				assignedOrdersChan <- assignedOrders
			case PROCESSES_FLAG:
				trimMessage := bytes.Trim(message[1:], "\x00")
				var processes []Process
				err = json.Unmarshal(trimMessage, &processes)
				if err != nil {
					fmt.Printf("Error unmarshalling master message, got: %s\n", trimMessage)
				}
				processesChan <- processes
			case CONFIRMATION_FLAG:
				slavesConsistentChan <- true
			}
		}

	}
}

func (n *NetworkNode) slaveRun(
	masterConn net.Conn,
	newOrderChan,
	finishedOrderChan chan types.Order, // Two-directional channel because listenToMaster forwards messages to this thread
	lightOnChan,
	lightOffChan chan<- types.Order,
	stateChan chan elevator.Elevator,
	assignedOrderChan chan<- types.Order) {

	slavesConsistentChan := make(chan bool)
	masterUnreachableChan := make(chan bool)
	assignedOrdersChan := make(chan []AssignedOrder)
	processesChan := make(chan []Process)

	go listenToMaster(masterConn, assignedOrdersChan, slavesConsistentChan, processesChan, masterUnreachableChan)

	fmt.Printf("Running slave %d.\n", n.Id)
	for {
		select {
		case order := <-newOrderChan:
			if !contains(n.AssignedOrders, order) {
				toSend, err := json.Marshal(order)
				if err == nil {
					buf := append([]byte{NEW_ORDER_FLAG}, toSend...)
					err := patientWrite(masterConn, buf, MASTER_RESPONSE_TIMEOUT)
					if err != nil {
						fmt.Printf("Reinitializing after failing to send new order.\n")
						n.ReinitializeNode(newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
					}
				} else {
					fmt.Printf("Failed to marshal new order.\n")
				}
			}
		case order := <-finishedOrderChan:
			toSend, err := json.Marshal(order)
			if err == nil {
				buf := append([]byte{FINISHED_ORDER_FLAG}, toSend...)
				err := patientWrite(masterConn, buf, MASTER_RESPONSE_TIMEOUT)
				if err != nil {
					fmt.Printf("Reinitializing after failing to send finished order.\n")
					n.ReinitializeNode(newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
				}
			} else {
				fmt.Printf("Failed to marshal finished order.\n")
			}
		case elevator := <-stateChan:
			toSend, err := json.Marshal(elevator)
			if err == nil {
				buf := append([]byte{ELEVATOR_STATE_FLAG}, toSend...)
				err := patientWrite(masterConn, buf, MASTER_RESPONSE_TIMEOUT)
				if err != nil {
					fmt.Printf("Reinitializing after failing to send new state.\n")
					n.ReinitializeNode(newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
				}
			} else {
				fmt.Printf("Failed to marshal state\n")
			}
		case <-slavesConsistentChan:
			diff := recentlyAssignedOrders(n.AssignedOrders, n.PreviousAssignedOrders)
			for _, order := range diff {
				lightOnChan <- order.Order
				if order.Id == n.Id {
					assignedOrderChan <- order.Order
				}
			}
			diff = recentlyAssignedOrders(n.PreviousAssignedOrders, n.AssignedOrders)
			for _, order := range diff {
				lightOffChan <- order.Order
			}
			n.PreviousAssignedOrders = append([]AssignedOrder{}, n.AssignedOrders...)
		case orders := <-assignedOrdersChan:
			n.AssignedOrders = orders
			toSend, err := json.Marshal(orders)
			if err == nil {
				buf := append([]byte{ASSIGNED_ORDERS_FLAG}, toSend...)
				err := patientWrite(masterConn, buf, MASTER_RESPONSE_TIMEOUT)
				if err != nil {
					fmt.Printf("Reinitializing after failing to send assigned orders.\n")
					n.ReinitializeNode(newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
				}
			} else {
				fmt.Printf("Failed to marshal received AssignedOrders.\n")
			}
		case processes := <-processesChan:
			n.Processes = processes
		case <-masterUnreachableChan:
			fmt.Printf("Reinitializing because master is unreachable.\n")
			n.ReinitializeNode(newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
		}
	}
}
