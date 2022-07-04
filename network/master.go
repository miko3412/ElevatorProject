package network

import (
	"encoding/json"
	"fmt"
	"net"
	"project-group-81/elevator"
	"project-group-81/types"
	"reflect"
	"sync"
	"time"

	"github.com/projecthunt/reuseable"
)

var mutex sync.Mutex

func (n *NetworkNode) broadcastMaster() {
	fmt.Printf("Master broadcasting socket to new nodes.\n")
	broadcastAddress := fmt.Sprintf("%s:%s", BROADCAST_ADDRESS, POLISH_POPE_DEATH_PORT)
	localAddress := fmt.Sprintf(":%s", n.getOwnProcess().Socket.Port)
	for {
		connection, err := reuseable.Dial(BROADCAST_NETWORK, localAddress, broadcastAddress)
		if err == nil {
			connection.Write([]byte(connection.LocalAddr().String()))
		} else if !interfaceAvailable() {
			return
		}
		time.Sleep(MASTER_BROADCAST_PERIOD)
	}
}

func (n *NetworkNode) listenForConnections(
	slaveConnections map[int]net.Conn,
	consistentSlaves map[int]bool,
	slaveMessageChan chan<- []byte) {
	mainSocket := n.getOwnProcess().Socket.String()
	listener, _ := reuseable.Listen(NETWORK, mainSocket)
	for {
		if !interfaceAvailable() {
			return
		}
		fmt.Printf("Master is listening for new slave on: %s\n", mainSocket)
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection in listenForConnections: %v", err)
			if !interfaceAvailable() {
				return
			}
		}
		fmt.Printf("Slave connected: %s\n", conn.RemoteAddr().String())

		// Read elevator socket from slave
		buf := make([]byte, MESSAGE_BUFFER_LENGTH)
		length, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("Error reading elevator socket from slave.\n")
		}
		elevatorSocket := FromString(string(buf[:length]))
		if !elevatorSocket.Valid() {
			continue
		}

		mutex.Lock()
		id := 0
		found := false
		for i, process := range n.Processes {
			if process.ElevatorSocket.Equals(elevatorSocket) {
				id = process.Id
				process.Active = true
				n.Processes[i] = process
				slaveConnections[process.Id] = conn
				consistentSlaves[process.Id] = false // Newly accepted node is by default not consistent
				found = true
				break
			}
		}
		if !found {
			id = len(n.Processes)
			slaveProcess := Process{
				Id:             id,
				Socket:         FromString(conn.RemoteAddr().(*net.TCPAddr).String()),
				Active:         true,
				ElevatorSocket: elevatorSocket,
				Elevator:       elevator.DefaultElevator()}
			n.Processes = append(n.Processes, slaveProcess)
			slaveConnections[slaveProcess.Id] = conn
			consistentSlaves[slaveProcess.Id] = false
		}

		message, err := json.Marshal(n.Processes)
		if err != nil {
			fmt.Printf("Error marshalling processes: %#v\n", err)
		}
		mutex.Unlock()

		_, err = conn.Write(message)
		if err != nil {
			fmt.Printf("Error writing slaveProcess to slave: %#v\n", err)
		}

		go listenToSlave(id, conn, slaveMessageChan)
	}
}

func listenToSlave(id int, slaveConn net.Conn, slaveMessageChan chan<- []byte) {
	buf := make([]byte, MESSAGE_BUFFER_LENGTH)
	for {
		length, err := slaveConn.Read(buf)
		if err != nil {
			fmt.Printf("Failed to receive from slave.\n")
			return
		}
		for _, message := range decode(buf[:length]) {
			go func(message []byte) {
				slaveMessageChan <- append([]byte{byte(id)}, message...)
			}(message)
		}
	}
}

func (n *NetworkNode) deleteNode(idToDelete int, slaveConnections map[int]net.Conn) {
	mutex.Lock()
	defer mutex.Unlock()
	for i, p := range n.Processes {
		if p.Id == idToDelete {
			p.Active = false
			n.Processes[i] = p
			break
		}
	}
	delete(slaveConnections, idToDelete)
}

func (n *NetworkNode) sendToSlaves(
	flag byte,
	message []byte,
	slaveConnections map[int]net.Conn,
	slaveMessageChan chan []byte) {
	toSend := append([]byte{flag}, message...)
	for id, conn := range slaveConnections {
		conn.SetWriteDeadline(time.Now().Add(SLAVE_WRITE_TIMEOUT))
		err := sendEncoded(conn, toSend)
		if err != nil {
			fmt.Printf("Failed to connect to slave %d.\n", id)
			n.deleteNode(id, slaveConnections)
			n.reassignOrders()
			message, err := json.Marshal(n.AssignedOrders)
			if err != nil {
				fmt.Print("Warning: Failed to marshal AssignedOrders\n")
			}
			n.sendToSlaves(ASSIGNED_ORDERS_FLAG, message, slaveConnections, slaveMessageChan)
		}
	}
	// Also send to local elevator
	go func(toSend []byte, Id int) {
		slaveMessageChan <- append([]byte{byte(Id)}, toSend...)
	}(toSend, n.Id)
}

// Receives messages from the network and forwards them to the right channel
func (n *NetworkNode) forwardMessages(
	slaveMessageChan <-chan []byte,
	assignedOrderDumpChan chan<- []byte,
	newOrderChan,
	finishedOrderChan chan<- types.Order,
	nodeStateChan chan<- []byte) {

	for {
		slaveMessage := <-slaveMessageChan
		if len(slaveMessage) < 2 {
			continue
		}
		slaveId := int(slaveMessage[0])
		flag := slaveMessage[1]
		trimmedMessage := slaveMessage[2:]
		switch flag {
		case ASSIGNED_ORDERS_FLAG:
			assignedOrderDumpChan <- append([]byte{byte(slaveId)}, trimmedMessage...)
		case NEW_ORDER_FLAG:
			var order types.Order
			err := json.Unmarshal(trimmedMessage, &order)
			if err == nil {
				newOrderChan <- order
			}
		case FINISHED_ORDER_FLAG:
			var order types.Order
			err := json.Unmarshal(trimmedMessage, &order)
			if err == nil {
				finishedOrderChan <- order
			}
		case ELEVATOR_STATE_FLAG:
			nodeStateChan <- append([]byte{byte(slaveId)}, trimmedMessage...)
		}
	}
}

func (n *NetworkNode) masterRun(
	newOrderChan,
	finishedOrderChan chan types.Order,
	lightOnChan, lightOffChan chan<- types.Order,
	stateChan chan elevator.Elevator,
	assignedOrderChan chan<- types.Order) {

	infoTimer := time.NewTimer(MASTER_INFO_PERIOD)
	slaveConnections := make(map[int]net.Conn)
	assignedOrderDumpChan := make(chan []byte)
	slaveMessageChan := make(chan []byte)
	nodeStateChan := make(chan []byte)
	consistentSlaves := make(map[int]bool)
	n.PreviousAssignedOrders = []AssignedOrder{} // Pretend that all assigned orders are new
	go n.broadcastMaster()
	go n.listenForConnections(slaveConnections, consistentSlaves, slaveMessageChan)
	go n.forwardMessages(slaveMessageChan, assignedOrderDumpChan, newOrderChan, finishedOrderChan, nodeStateChan)

	mutex.Lock()
	for i, process := range n.Processes {
		if process.Id != n.Id {
			process.Active = false
			n.Processes[i] = process
		}
	}
	mutex.Unlock()
	n.reassignOrders()
	message, err := json.Marshal(n.AssignedOrders)
	if err != nil {
		fmt.Print("Warning: Failed to marshal AssignedOrders\n")
	}
	n.sendToSlaves(ASSIGNED_ORDERS_FLAG, message, slaveConnections, slaveMessageChan)
	for {
		if !interfaceAvailable() {
			n.ReinitializeNode(newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
		}
		select {
		case assignedOrderDump := <-assignedOrderDumpChan:
			slaveId := int(assignedOrderDump[0])
			assignedOrderDump = assignedOrderDump[1:]
			toCompare, err := json.Marshal(n.AssignedOrders)
			if err != nil {
				fmt.Print("Warning: Failed to marshal AssignedOrders\n")
			}
			if reflect.DeepEqual(assignedOrderDump, toCompare) {
				mutex.Lock()
				consistentSlaves[slaveId] = true
				if activeSlavesConsistent(n.Processes, consistentSlaves) {
					fmt.Printf("Active slaves are consistent. Sending light confirmation message.\n")
					for consistentSlave := range consistentSlaves {
						consistentSlaves[consistentSlave] = false
					}
					// Sending consistent state confirmation
					confirmationMessage, err := json.Marshal(true)
					if err != nil {
						fmt.Print("Warning: Failed to marshal confirmation message\n")
					}
					n.sendToSlaves(CONFIRMATION_FLAG, confirmationMessage, slaveConnections, slaveMessageChan)
					// Updating lights for own local elevator
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
				}
				mutex.Unlock()
			} else {
				fmt.Printf("Received inconsistent assigned orders from node %d. Resending to everyone.\n", slaveId)
				message, err := json.Marshal(n.AssignedOrders)
				if err != nil {
					fmt.Print("Warning: Failed to marshal AssignedOrders\n")
				}
				n.sendToSlaves(ASSIGNED_ORDERS_FLAG, message, slaveConnections, slaveMessageChan)
			}
		case <-infoTimer.C:
			processesBlob, err := json.Marshal(n.Processes)
			if err != nil {
				fmt.Print("Warning: Failed to marshal Processes\n")
			}
			n.sendToSlaves(PROCESSES_FLAG, processesBlob, slaveConnections, slaveMessageChan)
			infoTimer.Reset(MASTER_INFO_PERIOD)
		case order := <-newOrderChan:
			if !contains(n.AssignedOrders, order) {
				assignedOrder := Assign(order, n.Processes)
				n.AssignedOrders = append(n.AssignedOrders, assignedOrder)
				message, err := json.Marshal(n.AssignedOrders)
				if err != nil {
					fmt.Print("Warning: Failed to marshal AssignedOrders\n")
				}
				n.sendToSlaves(ASSIGNED_ORDERS_FLAG, message, slaveConnections, slaveMessageChan)
			}
		case order := <-finishedOrderChan:
			for i, assignedOrder := range n.AssignedOrders {
				if assignedOrder.Order == order {
					n.AssignedOrders = append(n.AssignedOrders[:i], n.AssignedOrders[i+1:]...)
				}
			}
			lightOffChan <- order
			message, err := json.Marshal(n.AssignedOrders)
			if err != nil {
				fmt.Print("Warning: Failed to marshal AssignedOrders\n")
			}
			n.sendToSlaves(ASSIGNED_ORDERS_FLAG, message, slaveConnections, slaveMessageChan)
		case elevator := <-stateChan: // New state from local elevator
			for _, p := range n.Processes {
				if p.Id == n.Id {
					p.Elevator = elevator
					n.Processes[n.Id] = p
				}
			}
		case newNodeStateBlob := <-nodeStateChan: // New state for other elevator
			newNodeStateId := int(newNodeStateBlob[0])
			var elevator elevator.Elevator
			err := json.Unmarshal(newNodeStateBlob[1:], &elevator)
			if err != nil {
				fmt.Printf("Failed to unmarshal new slave state %#v\n", elevator)
			} else {
				mutex.Lock()
				for i, p := range n.Processes {
					if p.Id == newNodeStateId {
						p.Elevator = elevator
						n.Processes[i] = p
					}
				}
				mutex.Unlock()
			}
		}
	}
}
