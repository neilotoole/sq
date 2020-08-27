package json_test

import (
	stdj "encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStdjDecoder(t *testing.T) {
	f, err := os.Open("testdata/jsonl_actor_nested.jsonl")
	require.NoError(t, err)
	defer f.Close()

	dec := stdj.NewDecoder(f)
	dec.UseNumber()

	var tok stdj.Token

	for dec.More() {
		tok, err = dec.Token()
		require.NoError(t, err)
		t.Logf("%#v", tok)
	}

}
func TestStdjDecoder2(t *testing.T) {
	f, err := os.Open("testdata/jsonl_actor_nested.jsonl")
	require.NoError(t, err)
	defer f.Close()

	dec := stdj.NewDecoder(f)
	dec.UseNumber()

	var m map[string]interface{}

	for {
		err = dec.Decode(&m)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		fmt.Println(m)
	}

	//var tok stdj.Token
	//
	//for dec.More() {
	//	tok, err = dec.Token()
	//	require.NoError(t, err)
	//	t.Logf("%#v", tok)
	//}

}
func TestStdjDecoder3(t *testing.T) {
	f, err := os.Open("testdata/actor.json")
	require.NoError(t, err)
	defer f.Close()

	dec := stdj.NewDecoder(f)
	dec.UseNumber()

	var rawMsgs []stdj.RawMessage

	for {
		err = dec.Decode(&rawMsgs)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}

	for i, raw := range rawMsgs {
		fmt.Println(i, string(raw))
	}

	//var tok stdj.Token
	//
	//for dec.More() {
	//	tok, err = dec.Token()
	//	require.NoError(t, err)
	//	t.Logf("%#v", tok)
	//}

}
