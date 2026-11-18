// Package thriftfake is an in-process fake of the Databricks SQL warehouse
// endpoint, for testing the databricks driver without a workspace.
//
// It is an httptest server that speaks just enough of the Thrift TCLIService
// protocol (Thrift binary protocol over HTTP) for databricks-sql-go to open a
// session, execute a statement, and read a column-based result. The
// databricks-sql-go repo has a similar fake, but it, and the Thrift bindings
// it needs, are in an internal package that can't be imported. Rather than
// vendor ~30k lines of generated bindings, this package encodes and decodes
// the handful of messages involved using the apache/thrift protocol
// primitives directly. The field IDs come from TCLIService.thrift (and the
// Databricks extensions to it).
//
// The fake is not a query engine. A [Handler] maps each statement it receives
// to a canned [Result]. That is enough to exercise the driver's connection,
// type mapping, record scanning and error paths, but not the semantics of
// the SQL itself.
package thriftfake

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
)

// TypeID is a Thrift TTypeId, the type of a result column.
type TypeID int32

// The TTypeId values.
const (
	TypeBoolean         TypeID = 0
	TypeTinyInt         TypeID = 1
	TypeSmallInt        TypeID = 2
	TypeInt             TypeID = 3
	TypeBigInt          TypeID = 4
	TypeFloat           TypeID = 5
	TypeDouble          TypeID = 6
	TypeString          TypeID = 7
	TypeTimestamp       TypeID = 8
	TypeBinary          TypeID = 9
	TypeArray           TypeID = 10
	TypeMap             TypeID = 11
	TypeStruct          TypeID = 12
	TypeUnion           TypeID = 13
	TypeUserDefined     TypeID = 14
	TypeDecimal         TypeID = 15
	TypeNull            TypeID = 16
	TypeDate            TypeID = 17
	TypeVarchar         TypeID = 18
	TypeChar            TypeID = 19
	TypeIntervalYearMon TypeID = 20
	TypeIntervalDayTime TypeID = 21
)

// Column describes a result column.
type Column struct {
	Name string
	Type TypeID
}

// Result is the canned outcome of a statement.
//
// Each element of a row in Rows must suit the column type: bool for
// TypeBoolean; int8, int16, int32 and int64 for the matching integer types;
// float64 for TypeFloat and TypeDouble; []byte for TypeBinary; and string for
// every other type (dates, timestamps and decimals are strings on the wire).
// A nil element is a NULL.
type Result struct {
	// ErrMessage, if non-empty, makes the statement fail, with this
	// message and ErrSQLState.
	ErrMessage string

	// ErrSQLState is the SQLSTATE reported with ErrMessage.
	ErrSQLState string

	// Columns is the result schema. If it is empty, the statement has
	// no result set (like DDL or DML).
	Columns []Column

	// Rows is the result data.
	Rows [][]any
}

// Request is a statement received by the fake.
type Request struct {
	// Statement is the SQL text.
	Statement string

	// Params are the values of the positional parameters, in order, as
	// received (the Thrift TSparkParameterValue string value).
	Params []string
}

// Namespace is the initial catalog and schema that a client asked for when
// it opened a session. A field is empty if the client didn't ask for one.
type Namespace struct {
	Catalog string
	Schema  string
}

// Handler returns the Result for req.
type Handler func(req Request) Result

// Server is a running fake warehouse.
type Server struct {
	*httptest.Server
	handler Handler
	ops     map[string]*operation
	token   string
	reqs    []Request
	nss     []Namespace
	nextOp  int
	mu      sync.Mutex
}

// Option configures a Server.
type Option func(*Server)

// WithToken makes the server require the personal access token tok, sent as
// "Authorization: Bearer tok". Any other request is answered with 401.
func WithToken(tok string) Option {
	return func(s *Server) { s.token = tok }
}

type operation struct {
	result Result
}

// New starts a fake warehouse that uses h to answer statements. The server is
// closed when tb finishes.
func New(tb testing.TB, h Handler, opts ...Option) *Server {
	tb.Helper()
	s := &Server{handler: h, ops: map[string]*operation{}}
	for _, opt := range opts {
		opt(s)
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	tb.Cleanup(s.Close)
	return s
}

// HostPort returns the server's "host:port", e.g. "127.0.0.1:34567".
func (s *Server) HostPort() string {
	return s.Listener.Addr().String()
}

// Host returns the server's host, e.g. "127.0.0.1".
func (s *Server) Host() string {
	host, _, _ := net.SplitHostPort(s.HostPort())
	return host
}

// Sessions returns the initial namespace of each session opened so far,
// in order.
func (s *Server) Sessions() []Namespace {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Namespace(nil), s.nss...)
}

// Requests returns the statements received so far, in order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.reqs...)
}

// Statements returns the SQL text of the statements received so far.
func (s *Server) Statements() []string {
	reqs := s.Requests()
	stmts := make([]string, len(reqs))
	for i, req := range reqs {
		stmts[i] = req.Statement
	}
	return stmts
}

// Thrift field IDs. Those above 1280 are Databricks extensions.
const (
	fStatusCode     int16 = 1 // TStatus.statusCode
	fStatusErrMsg   int16 = 5 // TStatus.errorMessage
	fStatusSQLState int16 = 3 // TStatus.sqlState

	fExecStatement  int16 = 2    // TExecuteStatementReq.statement
	fExecParameters int16 = 1288 // TExecuteStatementReq.parameters
	fParamValue     int16 = 4    // TSparkParameter.value
	fParamStringVal int16 = 1    // TSparkParameterValue.stringValue

	fOpHandle int16 = 1 // the operation handle in the Get*/Close* requests

	fInitialNamespace int16 = 1284 // TOpenSessionReq.initialNamespace

	protocolV8 int32 = 42248 // SPARK_CLI_SERVICE_PROTOCOL_V8

	opStateFinished int32 = 2
	opStateError    int32 = 5

	// Thrift enum TSparkRowSetType.COLUMN_BASED_SET.
	rowSetColumnBased int32 = 1
)

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if s.token != "" && r.Header.Get("Authorization") != "Bearer "+s.token {
		http.Error(w, "invalid access token", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	in := thrift.NewTMemoryBuffer()
	if _, err = in.Write(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ip := thrift.NewTBinaryProtocolConf(in, &thrift.TConfiguration{})

	name, _, seqID, err := ip.ReadMessageBegin(ctx)
	if err != nil {
		// Not a Thrift call, e.g. a feature-flag or telemetry request.
		http.NotFound(w, r)
		return
	}
	args, err := readStruct(ctx, ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// The args of a Thrift method call are a struct whose field 1 is the
	// request struct.
	req, _ := args[1].(map[int16]any)
	resp := s.dispatch(name, req)

	out := thrift.NewTMemoryBuffer()
	op := thrift.NewTBinaryProtocolConf(out, &thrift.TConfiguration{})
	if err = op.WriteMessageBegin(ctx, name, thrift.REPLY, seqID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// The reply is a struct with the response at field 0.
	if err = writeValue(ctx, op, strct{{0, resp}}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = op.WriteMessageEnd(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = op.Flush(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-thrift")
	_, _ = w.Write(out.Bytes())
}

// dispatch returns the response struct for the Thrift method name.
func (s *Server) dispatch(name string, args map[int16]any) strct {
	switch name {
	case "OpenSession":
		ns, _ := args[fInitialNamespace].(map[int16]any)
		cat, _ := ns[1].([]byte)
		sch, _ := ns[2].([]byte)
		s.mu.Lock()
		s.nss = append(s.nss, Namespace{Catalog: string(cat), Schema: string(sch)})
		s.mu.Unlock()

		return strct{
			{1, okStatus()},
			{2, protocolV8},
			{3, strct{{1, handleID(opID(0))}}},
		}

	case "ExecuteStatement":
		return s.execute(args)

	case "GetOperationStatus":
		op := s.opFrom(args)
		if op == nil {
			return strct{{1, errStatus("unknown operation", "")}}
		}
		return operationStatus(op.result)

	case "GetResultSetMetadata":
		op := s.opFrom(args)
		if op == nil {
			return strct{{1, errStatus("unknown operation", "")}}
		}
		return strct{
			{1, okStatus()},
			{2, schemaOf(op.result.Columns)},
			{1281, rowSetColumnBased},
		}

	case "FetchResults":
		op := s.opFrom(args)
		if op == nil {
			return strct{{1, errStatus("unknown operation", "")}}
		}
		return strct{
			{1, okStatus()},
			{2, false}, // hasMoreRows
			{3, rowSetOf(op.result)},
		}

	default:
		// CloseOperation, CancelOperation, CloseSession and the rest.
		return strct{{1, okStatus()}}
	}
}

// execute handles ExecuteStatement: it records the request, asks the
// handler for a result, and returns an operation handle for it.
func (s *Server) execute(args map[int16]any) strct {
	req := Request{}
	if b, ok := args[fExecStatement].([]byte); ok {
		req.Statement = string(b)
	}
	if params, ok := args[fExecParameters].([]any); ok {
		for _, p := range params {
			pm, _ := p.(map[int16]any)
			vm, _ := pm[fParamValue].(map[int16]any)
			sv, _ := vm[fParamStringVal].([]byte)
			req.Params = append(req.Params, string(sv))
		}
	}

	res := s.handler(req)

	s.mu.Lock()
	s.reqs = append(s.reqs, req)
	s.nextOp++
	id := opID(s.nextOp)
	s.ops[id] = &operation{result: res}
	s.mu.Unlock()

	resp := strct{
		{1, okStatus()},
		{2, strct{
			{1, handleID(id)},
			{2, int32(0)}, // operationType: EXECUTE_STATEMENT
			{3, len(res.Columns) > 0},
		}},
	}

	// Like a real warehouse does for a small result, return everything as
	// "direct results" so that the client doesn't poll for the operation
	// status (which it does at a one-second interval) or fetch the rows.
	direct := strct{{1, operationStatus(res)}}
	if res.ErrMessage == "" {
		if len(res.Columns) > 0 {
			direct = append(direct,
				fld{2, strct{
					{1, okStatus()},
					{2, schemaOf(res.Columns)},
					{1281, rowSetColumnBased},
				}},
				fld{3, strct{
					{1, okStatus()},
					{2, false}, // hasMoreRows
					{3, rowSetOf(res)},
				}},
			)
		}
		direct = append(direct, fld{4, strct{{1, okStatus()}}}) // closeOperation
	}
	return append(resp, fld{1281, direct})
}

// operationStatus returns the TGetOperationStatusResp for res.
func operationStatus(res Result) strct {
	if res.ErrMessage != "" {
		return strct{
			{1, okStatus()},
			{2, opStateError},
			{3, res.ErrSQLState},
			{5, res.ErrMessage},
			// databricks-sql-go builds the error text from the display
			// message, not the error message.
			{1281, res.ErrMessage},
		}
	}
	return strct{
		{1, okStatus()},
		{2, opStateFinished},
		{9, len(res.Columns) > 0},
	}
}

// opFrom returns the operation that args refers to via its operation handle.
func (s *Server) opFrom(args map[int16]any) *operation {
	h, _ := args[fOpHandle].(map[int16]any)
	oid, _ := h[1].(map[int16]any)
	guid, _ := oid[1].([]byte)

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ops[string(guid)]
}

// opID returns a 16-byte operation GUID derived from n. The client
// expects handle GUIDs to be 16 bytes, like a UUID.
func opID(n int) string {
	var b [16]byte
	binary.BigEndian.PutUint64(b[8:], uint64(n)) //nolint:gosec // test helper, n is small and positive
	return string(b[:])
}

func okStatus() strct {
	return strct{{fStatusCode, int32(0)}}
}

func errStatus(msg, sqlState string) strct {
	return strct{
		{fStatusCode, int32(3)},
		{fStatusSQLState, sqlState},
		{fStatusErrMsg, msg},
	}
}

// handleID returns a THandleIdentifier for id.
func handleID(id string) strct {
	return strct{{1, []byte(id)}, {2, []byte("secret")}}
}

// schemaOf returns the TTableSchema for cols.
func schemaOf(cols []Column) strct {
	descs := make([]any, len(cols))
	for i, col := range cols {
		descs[i] = strct{
			{1, col.Name},
			{2, strct{{1, list{thrift.STRUCT, []any{
				strct{{1, strct{{1, int32(col.Type)}}}},
			}}}}},
			{3, int32(i)},
		}
	}
	return strct{{1, list{thrift.STRUCT, descs}}}
}

// rowSetOf returns a column-based TRowSet for res.
func rowSetOf(res Result) strct {
	cols := make([]any, len(res.Columns))
	for i, col := range res.Columns {
		vals := make([]any, len(res.Rows))
		for r, row := range res.Rows {
			vals[r] = row[i]
		}
		cols[i] = columnOf(col.Type, vals)
	}

	return strct{
		{1, int64(0)}, // startRowOffset
		{2, list{thrift.STRUCT, nil}},
		{3, list{thrift.STRUCT, cols}},
	}
}

// columnOf returns the TColumn (a union) holding vals for a column of type t.
func columnOf(t TypeID, vals []any) strct {
	var (
		fieldID int16
		elem    thrift.TType
		zero    any
	)
	switch t { //nolint:exhaustive // every other type is a string on the wire
	case TypeBoolean:
		fieldID, elem, zero = 1, thrift.BOOL, false
	case TypeTinyInt:
		fieldID, elem, zero = 2, thrift.BYTE, int8(0)
	case TypeSmallInt:
		fieldID, elem, zero = 3, thrift.I16, int16(0)
	case TypeInt:
		fieldID, elem, zero = 4, thrift.I32, int32(0)
	case TypeBigInt:
		fieldID, elem, zero = 5, thrift.I64, int64(0)
	case TypeFloat, TypeDouble:
		fieldID, elem, zero = 6, thrift.DOUBLE, float64(0)
	case TypeBinary:
		fieldID, elem, zero = 8, thrift.STRING, []byte{}
	default:
		fieldID, elem, zero = 7, thrift.STRING, ""
	}

	nulls := make([]byte, (len(vals)+7)/8)
	out := make([]any, len(vals))
	for i, v := range vals {
		if v == nil {
			nulls[i/8] |= 1 << (i % 8)
			out[i] = zero
			continue
		}
		out[i] = v
	}

	return strct{{fieldID, strct{
		{1, list{elem, out}},
		{2, nulls},
	}}}
}

// strct is a Thrift struct: an ordered list of fields. A field value
// is one of bool, int8, int16, int32, int64, float64, string, []byte, list
// or strct. A Go int is written as an i32.
type strct []fld

type fld struct { //nolint:govet // field alignment: reads as {id, value}
	id  int16
	val any
}

// list is a Thrift list whose elements have the Thrift type elem.
type list struct { //nolint:govet // field alignment: reads as {elem type, values}
	elem thrift.TType
	vals []any
}

// writeValue writes v in Thrift binary encoding.
func writeValue(ctx context.Context, p thrift.TProtocol, v any) error {
	switch v := v.(type) {
	case bool:
		return p.WriteBool(ctx, v)
	case int8:
		return p.WriteByte(ctx, v)
	case int16:
		return p.WriteI16(ctx, v)
	case int32:
		return p.WriteI32(ctx, v)
	case int:
		return p.WriteI32(ctx, int32(v)) //nolint:gosec // test helper
	case int64:
		return p.WriteI64(ctx, v)
	case float64:
		return p.WriteDouble(ctx, v)
	case string:
		return p.WriteString(ctx, v)
	case []byte:
		return p.WriteBinary(ctx, v)
	case list:
		if err := p.WriteListBegin(ctx, v.elem, len(v.vals)); err != nil {
			return err
		}
		for _, e := range v.vals {
			if err := writeValue(ctx, p, e); err != nil {
				return err
			}
		}
		return p.WriteListEnd(ctx)
	case strct:
		if err := p.WriteStructBegin(ctx, "s"); err != nil {
			return err
		}
		for _, f := range v {
			if err := p.WriteFieldBegin(ctx, "f", thriftType(f.val), f.id); err != nil {
				return err
			}
			if err := writeValue(ctx, p, f.val); err != nil {
				return err
			}
			if err := p.WriteFieldEnd(ctx); err != nil {
				return err
			}
		}
		if err := p.WriteFieldStop(ctx); err != nil {
			return err
		}
		return p.WriteStructEnd(ctx)
	default:
		return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, io.ErrUnexpectedEOF)
	}
}

// thriftType returns the Thrift wire type for the Go value v.
func thriftType(v any) thrift.TType {
	switch v.(type) {
	case bool:
		return thrift.BOOL
	case int8:
		return thrift.BYTE
	case int16:
		return thrift.I16
	case int32, int:
		return thrift.I32
	case int64:
		return thrift.I64
	case float64:
		return thrift.DOUBLE
	case string, []byte:
		return thrift.STRING
	case list:
		return thrift.LIST
	default:
		return thrift.STRUCT
	}
}

// readStruct reads a Thrift struct into a map of field ID to value. Values
// are decoded as: bool, int8, int16, int32, int64, float64, []byte (for
// STRING, which may be text or binary), []any (lists and sets), and
// map[int16]any (structs). Maps are read and discarded.
func readStruct(ctx context.Context, p thrift.TProtocol) (map[int16]any, error) {
	if _, err := p.ReadStructBegin(ctx); err != nil {
		return nil, err
	}

	m := map[int16]any{}
	for {
		_, ft, id, err := p.ReadFieldBegin(ctx)
		if err != nil {
			return nil, err
		}
		if ft == thrift.STOP {
			break
		}

		v, err := readValue(ctx, p, ft)
		if err != nil {
			return nil, err
		}
		m[id] = v

		if err = p.ReadFieldEnd(ctx); err != nil {
			return nil, err
		}
	}

	return m, p.ReadStructEnd(ctx)
}

func readValue(ctx context.Context, p thrift.TProtocol, ft thrift.TType) (any, error) {
	switch ft {
	case thrift.BOOL:
		return p.ReadBool(ctx)
	case thrift.BYTE:
		return p.ReadByte(ctx)
	case thrift.I16:
		return p.ReadI16(ctx)
	case thrift.I32:
		return p.ReadI32(ctx)
	case thrift.I64:
		return p.ReadI64(ctx)
	case thrift.DOUBLE:
		return p.ReadDouble(ctx)
	case thrift.STRING:
		return p.ReadBinary(ctx)
	case thrift.STRUCT:
		return readStruct(ctx, p)
	case thrift.LIST, thrift.SET:
		var (
			et   thrift.TType
			size int
			err  error
		)
		if ft == thrift.LIST {
			et, size, err = p.ReadListBegin(ctx)
		} else {
			et, size, err = p.ReadSetBegin(ctx)
		}
		if err != nil {
			return nil, err
		}
		vals := make([]any, 0, size)
		for range size {
			v, err := readValue(ctx, p, et)
			if err != nil {
				return nil, err
			}
			vals = append(vals, v)
		}
		if ft == thrift.LIST {
			return vals, p.ReadListEnd(ctx)
		}
		return vals, p.ReadSetEnd(ctx)
	case thrift.MAP:
		kt, vt, size, err := p.ReadMapBegin(ctx)
		if err != nil {
			return nil, err
		}
		for range size {
			if _, err = readValue(ctx, p, kt); err != nil {
				return nil, err
			}
			if _, err = readValue(ctx, p, vt); err != nil {
				return nil, err
			}
		}
		return nil, p.ReadMapEnd(ctx)
	default:
		return nil, thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, bytes.ErrTooLarge)
	}
}
