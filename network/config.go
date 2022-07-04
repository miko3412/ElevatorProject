package network

import "time"

const (
	//Flags in messages sent from master
	ASSIGNED_ORDERS_FLAG byte = 0
	PROCESSES_FLAG       byte = 1
	CONFIRMATION_FLAG    byte = 2
	//Flags in messages sent from slave
	NEW_ORDER_FLAG      byte = 3
	FINISHED_ORDER_FLAG byte = 4
	ELEVATOR_STATE_FLAG byte = 5

	INTERFACE_NAME         string = "wlp1s0"
	NETWORK                string = "tcp4"
	BROADCAST_NETWORK      string = "udp"
	BROADCAST_ADDRESS      string = "255.255.255.255"
	POLISH_POPE_DEATH_PORT string = "2137"

	MASTER_RESPONSE_TIMEOUT = time.Second * 10 // Time before slave assumes master to be dead
	MASTER_SEARCH_TIMEOUT   = time.Second * 5  // Time before node is assumed not to be master
	SLAVE_WRITE_TIMEOUT     = time.Second * 2  // Time before master assumes slave to be dead

	MASTER_BROADCAST_PERIOD = time.Second     // Master network information broadcast period
	MASTER_INFO_PERIOD      = time.Second * 5 // Connected slave broadcast

	MASTER_PROMOTION_TIME = MASTER_SEARCH_TIMEOUT * 3 // Highest expected master promotion time

	MESSAGE_BUFFER_LENGTH = 2048
)

func PacketDelimiter() []byte {
	return []byte{'E', 'N', 'D'}
}
