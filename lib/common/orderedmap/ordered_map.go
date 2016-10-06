package orderedmap

import (
	"bytes"
	"encoding/json"
)

type KeyVal struct {
	Key string
	Val interface{}
}

// Define an ordered map
type Map struct {
	items []KeyVal
}

func (m *Map) Items() []KeyVal {
	return m.items
}

func (m *Map) Len() int {
	return len(m.items)
}

// Put adds a key/val to the ordered map, returning any existing value (which is
// overwritten).
func (m *Map) Put(key string, val interface{}) interface{} {

	kv := KeyVal{Key: key, Val: val}

	for i, item := range m.items {
		if item.Key == key {
			m.items[i] = kv
			return item.Val
		}
	}

	m.items = append(m.items, kv)
	return nil
}

// Implement the json.Marshaler interface
func (m *Map) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("{")
	for i, kv := range m.items {
		if i != 0 {
			buf.WriteString(",")
		}
		// marshal key
		key, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteString(":")
		// marshal value
		val, err := json.Marshal(kv.Val)
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}

	buf.WriteString("}")
	return buf.Bytes(), nil
}
