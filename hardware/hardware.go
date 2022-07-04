package hardware

import (
	"fmt"
	"net"
	"project-group-81/types"
	"sync"
)

type HardwareConn struct {
	conn  net.Conn
	mutex sync.Mutex
}

func boolToByte(b bool) byte {
	if b {
		return 1
	} else {
		return 0
	}
}

func DialHardware(rport int) (HardwareConn, error) {
	network := "tcp4"
	rsocket := fmt.Sprintf("localhost:%d", rport)
	conn, err := net.Dial(network, rsocket)
	return HardwareConn{conn: conn}, err
}

func (hc *HardwareConn) send(message []byte) (int, error) {
	return hc.conn.Write(message)
}

func (hc *HardwareConn) receive() ([]byte, error) {
	b := make([]byte, 4)
	_, err := hc.conn.Read(b)
	return b, err
}

func (hc *HardwareConn) request(message []byte) ([]byte, error) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	hc.send(message)
	return hc.receive()
}

func (hc *HardwareConn) WriteMotorDirection(md types.MotorDirection) {
	message := []byte{1, byte(md), 0, 0}
	hc.send(message)
}

func (hc *HardwareConn) WriteOrderButtonLight(o types.Order, on bool) {
	message := []byte{2, byte(o.C), byte(o.F), boolToByte(on)}
	hc.send(message)
}

func (hc *HardwareConn) WriteFloorIndicator(f types.Floor) {
	message := []byte{3, byte(f), 0, 0}
	hc.send(message)
}

func (hc *HardwareConn) WriteDoorOpenLight(on bool) {
	message := []byte{4, boolToByte(on), 0, 0}
	hc.send(message)
}

func (hc *HardwareConn) WriteStopButtonLight(on bool) {
	message := []byte{5, boolToByte(on), 0, 0}
	hc.send(message)
}

func (hc *HardwareConn) ReadOrderButton(o types.Order) bool {
	message := []byte{6, byte(o.C), byte(o.F), 0}
	response, err := hc.request(message)
	if err != nil {
		fmt.Printf("Hardware error in function ReadOrderButton: %v\n", err)
	}
	active := response[1] == 1
	return active
}

func (hc *HardwareConn) ReadFloorSensor() (bool, types.Floor) {
	message := []byte{7, 0, 0, 0}
	response, err := hc.request(message)
	if err != nil {
		fmt.Printf("Hardware error in function ReadFloorSensor: %v\n", err)
	}
	active := response[1] == 1
	floor := types.Floor(response[2])
	return active, floor
}

func (hc *HardwareConn) ReadStopButton() bool {
	message := []byte{8, 0, 0, 0}
	response, err := hc.request(message)
	if err != nil {
		fmt.Printf("Hardware error in function ReadStopButton: %v\n", err)
	}
	active := response[1] == 1
	return active
}

func (hc *HardwareConn) ReadObstructionSwitch() bool {
	message := []byte{9, 0, 0, 0}
	response, err := hc.request(message)
	if err != nil {
		fmt.Printf("Hardware error in function ReadObstructionSwitch: %v\n", err)
	}
	active := response[1] == 1
	return active
}
