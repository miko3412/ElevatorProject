package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
)

func sendEncoded(conn net.Conn, buf []byte) error {
	packet := NetworkPacket{Lenght: len(buf), Content: buf}
	packetBytes, err := json.Marshal(packet)
	if err == nil {
		_, err := conn.Write(append(packetBytes, []byte(PacketDelimiter())...))
		return err
	} else {
		fmt.Printf("Error in marshalling network packet before sending.\n")
		return err
	}
}

func decode(message []byte) [][]byte {
	validPackets := make([][]byte, 0)
	splittedPackets := bytes.Split(message, []byte(PacketDelimiter()))
	for _, packet1 := range splittedPackets {
		var packet2 NetworkPacket
		err := json.Unmarshal(packet1, &packet2)
		if err == nil {
			validPackets = append(validPackets, packet2.Content[:packet2.Lenght])
		} else if len(packet1) != 0 {
			fmt.Printf("Error in unmarshalling packet: %s\n", packet1)
		}
	}
	return validPackets
}
