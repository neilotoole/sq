package drvr

import "encoding/json"

type SourceMetadata struct {
	Handle             string  `json:"handle"`
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

type Column struct {
	Name         string `json:"name"`
	Position     int64  `json:"position"`
	PrimaryKey   bool   `json:"primary_key"`
	Datatype     string `json:"drvr.datatype"`
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
