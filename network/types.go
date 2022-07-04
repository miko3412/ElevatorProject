package network

import (
	"fmt"
	"project-group-81/elevator"
	"project-group-81/types"
	"strconv"
	"strings"
)

type NetworkPacket struct {
	Lenght  int
	Content []byte
}

type AssignedOrder struct {
	Id    int
	Order types.Order
}

type Socket struct {
	Address,
	Port string
}

func (s Socket) String() string {
	return fmt.Sprintf("%s:%s", s.Address, s.Port)
}

func FromString(s string) Socket {
	splits := strings.Split(s, ":")
	return Socket{splits[0], splits[1]}
}

func (s1 *Socket) Equals(s2 Socket) bool {
	return s1.String() == s2.String()
}

func (s *Socket) Valid() bool {
	_, err := strconv.Atoi(s.Port)
	return strings.Count(s.Address, ".") == 3 && err == nil
}

type Process struct {
	Id             int
	Socket         Socket
	Active         bool
	ElevatorSocket Socket
	Elevator       elevator.Elevator
}

type NetworkNode struct {
	Id                     int
	AssignedOrders         []AssignedOrder
	PreviousAssignedOrders []AssignedOrder
	Processes              []Process
}
