package main

import (
	"fmt"
	"os"
	"os/exec"
	"project-group-81/config"
	"project-group-81/elevator"
	"project-group-81/hardware"
	"project-group-81/network"
	"project-group-81/types"
	"strconv"
	"time"
)

func SpawnSimulator(hwPort int) error {
	return exec.Command("gnome-terminal", "--", "./SimElevatorServer", "--numfloors", strconv.Itoa(config.NUMBER_OF_FLOORS), "--port", strconv.Itoa(hwPort)).Run()
}

func SpawnElevator(hwPort int) error {
	return exec.Command("gnome-terminal", "--", "go", "run", ".", "single", strconv.Itoa(hwPort)).Run()
}

func Run(hwPort int) {
	fmt.Print("Connecting to hardware.\n")
	hc, err := hardware.DialHardware(hwPort)
	if err != nil {
		fmt.Printf("Failed to dial hardware: %v\n", err)
		return
	}

	newOrderChan := make(chan types.Order)
	finishedOrderChan := make(chan types.Order)
	stateChan := make(chan elevator.Elevator)
	assignedOrderChan := make(chan types.Order)
	lightOnChan := make(chan types.Order)
	lightOffChan := make(chan types.Order)

	ipAddress := network.GetIPAddress()
	hwSocket := network.Socket{Address: ipAddress, Port: fmt.Sprint(hwPort)}

	// Initializing network node
	go network.InitializeNode(hwSocket, newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
	elevator.RunElevator(&hc, newOrderChan, finishedOrderChan, lightOnChan, lightOffChan, stateChan, assignedOrderChan)
	SpawnElevator(hwPort)
}

func main() {
	args := os.Args[1:]
	if args[0] == "single" {
		hwPort, _ := strconv.Atoi(args[1])
		Run(hwPort)
	} else if args[0] == "system" {
		elevators, _ := strconv.Atoi(args[1])
		base_hwPort, _ := strconv.Atoi(args[2])
		fmt.Printf("Running with %d simulators.\n", elevators)
		for i := 0; i < elevators; i++ {
			SpawnSimulator(base_hwPort + i)
			SpawnElevator(base_hwPort + i)
			time.Sleep(time.Second * 5)
		}
	}
}
