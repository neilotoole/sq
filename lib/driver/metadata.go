package driver

import "encoding/json"

type SourceMetadata struct {
	Ref                string  `json:"ref"`
	Name               string  `json:"name"`
	FullyQualifiedName string  `json:"fq_name"`
	Location           string  `json:"location"`
	Size               int64   `json:"size"`
	Tables             []Table `json:"tables"`
}

func (d *SourceMetadata) String() string {
	bytes, _ := json.Marshal(d)
	return string(bytes)
}

type Table struct {
	Name     string   `json:"name"`
	RowCount int64    `json:"rows"`
	Size     int64    `json:"tbl_size"`
	Comment  string   `json:"comment,omitempty"`
	Columns  []Column `json:"columns"`
}

func (t *Table) String() string {
	bytes, _ := json.Marshal(t)
	return string(bytes)
}

//func (t *Table) ToOrderedMap() *OrderedMap {
//
//	m := &OrderedMap{}
//	m.Put("name", t.Name)
//	m.Put("row_count", t.Position)go inst
//	m.Put("size_bytes", t.Datatype)
//	m.Put("nullable", t.Nullable)
//	m.Put("key", t.Key)
//
//	m.Put("comment", t.Comment)
//
//	cols := []OrderedMap
//
//
//	return m
//}

type Column struct {
	Name         string `json:"name"`
	Position     int64  `json:"position"`
	PrimaryKey   bool   `json:"primary_key"`
	Datatype     string `json:"datatype"`
	ColType      string `json:"col_type"`
	Nullable     bool   `json:"nullable"`
	DefaultValue string `json:"default_value,omitempty"`
	//Key          string `json:"key"`
	//Extra        string `json:"extra"`
	Comment string `json:"comment,omitempty"`
}

func (c *Column) String() string {
	bytes, _ := json.Marshal(c)
	return string(bytes)
}

//func (c *Column) ToOrderedMap() *orderedmap.Map {
//
//	m := &orderedmap.Map{}
//	m.Put("name", c.Name)
//	m.Put("position", c.Position)
//	m.Put("datatype", c.Datatype)
//	m.Put("col_type", c.ColType)
//	m.Put("nullable", c.Nullable)
//	//m.Put("key", c.Key)
//	//m.Put("extra", c.Extra)
//	m.Put("comment", c.Comment)
//	return m
//}
