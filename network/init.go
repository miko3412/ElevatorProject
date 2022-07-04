package network

import (
	"encoding/json"
	"fmt"
	"net"
	"project-group-81/elevator"
	"project-group-81/types"
	"time"

	"github.com/projecthunt/reuseable"
)

func (n *NetworkNode) ReinitializeNode(
	newOrderChan,
	finishedOrderChan chan types.Order,
	lightOnChan,
	lightOffChan chan<- types.Order,
	stateChan chan elevator.Elevator,
	assignedOrderChan chan<- types.Order) {

	fmt.Printf("Reinitializing node.\n")

	for !interfaceAvailable() {

	}

	conn := n.findNewMaster()
	if conn == nil {
		fmt.Printf("Failed to find new master. Turning into master.\n")
		n.masterRun(newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
	} else {
		n.slaveRun(conn, newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
	}
}

func InitializeNode(
	hwSocket Socket,
	newOrderChan,
	finishedOrderChan chan types.Order, // Two-directional channels because listenToMaster forwards messages to this thread
	lightOnChan,
	lightOffChan chan<- types.Order,
	stateChan chan elevator.Elevator,
	assignedOrderChan chan<- types.Order) {

	fmt.Printf("Waiting %d seconds before initializing network node.\n", int(MASTER_PROMOTION_TIME.Seconds()))
	time.Sleep(MASTER_PROMOTION_TIME)

	freePort, _ := getFreePort()
	process := Process{0, Socket{hwSocket.Address, fmt.Sprint(freePort)}, true, hwSocket, elevator.DefaultElevator()}
	networkNode := NetworkNode{0, []AssignedOrder{}, []AssignedOrder{}, []Process{process}}

	for !interfaceAvailable() {
	} // Wait here till network interface is available

	lsocket := networkNode.getOwnProcess().Socket.String()
	rsocket, err := initialSearchForMaster()
	if err != nil {
		go networkNode.masterRun(newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
		return
	}

	fmt.Printf("Master suggests connection on socket %s\n", rsocket)
	conn, err := reuseable.DialTimeout(NETWORK, lsocket, rsocket, MASTER_RESPONSE_TIMEOUT)
	if err == nil {
		conn.SetWriteDeadline(time.Now().Add(MASTER_RESPONSE_TIMEOUT))
		_, err := conn.Write([]byte(hwSocket.String()))
		if err != nil {
			panic("Error writing hardware socket to master in InitializeNode.\n")
		}
		buf := make([]byte, MESSAGE_BUFFER_LENGTH)
		length, err := patientRead(conn, buf, MASTER_RESPONSE_TIMEOUT)
		if err != nil {
			panic("Error reading processes response from master in InitializeNode.\n")
		}
		err = json.Unmarshal(buf[:length], &networkNode.Processes)
		if err != nil {
			panic("Error unmarshalling processes response from master in InitializeNode.\n")
		}

		// Extract id if elevator was previously active
		for _, p := range networkNode.Processes {
			if p.ElevatorSocket.Equals(hwSocket) {
				networkNode.Id = p.Id
			}
		}

		// Resend assigned orders
		go func(orders types.OrderSet) {
			for order := range orders {
				if order.C == types.Car {
					assignedOrderChan <- order
				}
			}
		}(networkNode.getOwnProcess().Elevator.Orders)

		go networkNode.slaveRun(conn, newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
		return
	} else { // Should happen very rarely, only if death between broadcast and connection attempt
		fmt.Printf("Master died between broadcast and connection attempt!\n")
		go networkNode.masterRun(newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
		return
	}
}

func (n *NetworkNode) findNewMaster() net.Conn {
	lsocket := n.getOwnProcess().Socket.String()
	for _, process := range n.Processes {
		time.Sleep(3 * time.Second)
		rsocket := process.Socket.String()
		fmt.Printf("Trying to connect to %s\n", rsocket)
		if n.Id == process.Id {
			return nil
		} else {
			time.Sleep(2 * time.Second)
			if conn, err := reuseable.DialTimeout(NETWORK, lsocket, rsocket, MASTER_SEARCH_TIMEOUT); err == nil {
				return conn
			}
		}
	}
	return nil
}

func initialSearchForMaster() (string, error) {
	fmt.Printf("Starting initial search for master.\n")
	localAddress := fmt.Sprintf(":%s", POLISH_POPE_DEATH_PORT)
	connection, err := reuseable.ListenPacket(BROADCAST_NETWORK, localAddress)
	if err != nil {
		fmt.Printf("ListenPacket error: %v\n", err)
		return "", err
	}
	buf := make([]byte, MESSAGE_BUFFER_LENGTH)
	for {
		connection.SetReadDeadline(time.Now().Add(MASTER_SEARCH_TIMEOUT))
		length, _, err := connection.ReadFrom(buf)
		if err == nil && length != 0 {
			fmt.Printf("Got initial message from master: %s\n", buf[:length])
			return string(buf[:length]), nil
		} else if err != nil {
			return "", err
		}
	}
}
