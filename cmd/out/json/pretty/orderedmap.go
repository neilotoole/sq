package pretty

type keyval struct {
	key string
	val interface{}
}

// An ordered map
// TODO: At some point in time omap was actually more like a map, but
// functionality has slowly been stripped away, hopefully this will go away
// entirely at some point.
type omap struct {
	kvs []keyval
}

func (m *omap) entries() []keyval {
	return m.kvs
}

func (m *omap) len() int {
	return len(m.kvs)
}

// Put adds a key/val to the ordered map, returning any existing value (which is
// overwritten).
func (m *omap) put(key string, val interface{}) interface{} {

	kv := keyval{key: key, val: val}

	for i, item := range m.kvs {
		if item.key == key {
			m.kvs[i] = kv
			return item.val
		}
	}

	m.kvs = append(m.kvs, kv)
	return nil
}

//
//// Implement the json.Marshaler interface
//func (m *Map) MarshalJSON() ([]byte, error) {
//	var buf bytes.Buffer
//
//	buf.WriteString("{")
//	for i, kv := range m.items {
//		if i != 0 {
//			buf.WriteString(",")
//		}
//		// marshal key
//		key, err := json.Marshal(kv.Key)
//		if err != nil {
//			return nil, err
//		}
//		buf.Write(key)
//		buf.WriteString(":")
//		// marshal value
//		val, err := json.Marshal(kv.Val)
//		if err != nil {
//			return nil, err
//		}
//		buf.Write(val)
//	}
//
//	buf.WriteString("}")
//	return buf.Bytes(), nil
//}
