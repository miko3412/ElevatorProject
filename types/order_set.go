package types

import (
	"encoding/json"
	"fmt"
)

type OrderSet map[Order]bool

func (os OrderSet) Remove(o Order) {
	delete(os, o)
}

func (os OrderSet) Insert(o Order) {
	os[o] = true
}

func (os OrderSet) Contains(o Order) bool {
	_, ok := os[o]
	return ok
}

func (os OrderSet) IsEmpty() bool {
	return len(os) == 0
}

func (os OrderSet) Print() {
	for o := range os {
		fmt.Printf("%s\n", o)
	}
}

// Maps cannot be marshalled because JSON doesn't accept integer keys (which "Order"-keys are converted to)
func (os OrderSet) MarshalJSON() ([]byte, error) {
	var orderSlice []Order
	for key := range os {
		orderSlice = append(orderSlice, key)
	}
	return json.Marshal(orderSlice)
}

func (os *OrderSet) UnmarshalJSON(b []byte) error {
	var orderSlice []Order
	*os = make(map[Order]bool)
	err := json.Unmarshal(b, &orderSlice)
	if err == nil {
		for _, o := range orderSlice {
			os.Insert(o)
		}
		return nil
	} else {
		return err
	}
}
