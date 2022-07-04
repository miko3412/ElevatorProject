package network

import (
	"fmt"
	"log"
	"net"
	"project-group-81/types"
	"strings"
	"time"
)

func interfaceAvailable() bool {
	byNameInterface, err := net.InterfaceByName(INTERFACE_NAME)
	if err != nil {
		fmt.Printf("Error, interface %s is not available", INTERFACE_NAME)
		return true
	}
	return strings.Contains(byNameInterface.Flags.String(), "up")
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func GetIPAddress() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}

func contains(assignedOrders []AssignedOrder, order types.Order) bool {
	for _, assignedOrder := range assignedOrders {
		if assignedOrder.Order == order {
			return true
		}
	}
	return false
}

func recentlyAssignedOrders(newSlice []AssignedOrder, oldSlice []AssignedOrder) []AssignedOrder {
	var difference []AssignedOrder
	for _, a1 := range newSlice {
		found := false
		for _, a2 := range oldSlice {
			if a1 == a2 {
				found = true
				break
			}
		}
		if !found {
			difference = append(difference, a1)
		}
	}
	return difference
}

func (n *NetworkNode) reassignOrders() {
	fmt.Printf("Reassigning orders.\n")
	aliveIds := make(map[int]bool)
	for _, process := range n.Processes {
		if process.Active {
			aliveIds[process.Id] = true
		}
	}
	for i, assignedOrder := range n.AssignedOrders {
		if _, found := aliveIds[assignedOrder.Id]; !found {
			assignedOrder = Assign(assignedOrder.Order, n.Processes)
			n.AssignedOrders[i] = assignedOrder
			// Pretend as if order is new by removing it from PreviousAssignedOrders
			for i, previousAssignOrder := range n.PreviousAssignedOrders {
				if previousAssignOrder.Order == assignedOrder.Order {
					n.PreviousAssignedOrders = append(n.PreviousAssignedOrders[:i], n.PreviousAssignedOrders[i+1:]...)
				}
			}
		}
	}
}

func activeSlavesConsistent(processes []Process, consistentSlaves map[int]bool) bool {
	allConsistent := true
	for _, node := range processes {
		if node.Active {
			allConsistent = allConsistent && consistentSlaves[node.Id]
		}
	}
	return allConsistent
}

func (n *NetworkNode) getOwnProcess() Process {
	for _, process := range n.Processes {
		if process.Id == n.Id {
			return process
		}
	}
	return Process{}
}

func patientWrite(conn net.Conn, message []byte, timeout time.Duration) error {
	for {
		conn.SetWriteDeadline(time.Now().Add(timeout))
		err := sendEncoded(conn, message)
		if err == nil {
			return nil
		} else if interfaceAvailable() {
			return err
		} else {
			continue
		}
	}
}

func patientRead(conn net.Conn, buf []byte, timeout time.Duration) (int, error) {
	for {
		conn.SetReadDeadline(time.Now().Add(timeout))
		length, err := conn.Read(buf)
		if err == nil {
			return length, nil
		} else if interfaceAvailable() {
			return length, err
		} else {
			continue
		}
	}
}
