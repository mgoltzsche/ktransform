package transform

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	testInputStringMap = map[string]string{
		"str": "value1",
		"num": "7",
		"obj": "prop: x",
	}
	testInputObjMap = map[string]interface{}{
		"str": "value1",
		"num": 7,
		"obj": map[string]interface{}{
			"prop": "x",
		},
		"nil": nil,
	}
	expectedStringMap = map[string]string{
		"str": "value1",
		"num": "7",
		"obj": `{"prop":"x"}`,
		"nil": "",
	}
	expectedInputMap = map[string]interface{}{
		"str": map[string]interface{}{"string": "value1", "object": map[string]interface{}(nil)},
		"num": map[string]interface{}{"string": "7", "object": map[string]interface{}(nil)},
		"obj": map[string]interface{}{"string": "prop: x", "object": map[string]interface{}{"prop": "x"}},
	}
)

func TestInputMapFromStringMap(t *testing.T) {
	a := InputMapFromStringMap(testInputStringMap)
	require.Equal(t, expectedInputMap, a)
}

func TestStringMapFromOutput(t *testing.T) {
	a, err := StringMapFromOutput(testInputObjMap)
	require.NoError(t, err)
	require.Equal(t, expectedStringMap, a)
}

func TestInputMapFromBytesMap(t *testing.T) {
	a := InputMapFromBytesMap(toBytes(testInputStringMap))
	require.Equal(t, expectedInputMap, a)
}

func TestBytesMapFromOutput(t *testing.T) {
	a, err := BytesMapFromOutput(testInputObjMap)
	require.NoError(t, err)
	require.Equal(t, toBytes(expectedStringMap), a)
}

func toBytes(m map[string]string) map[string][]byte {
	r := map[string][]byte{}
	for k, v := range m {
		r[k] = []byte(v)
	}
	return r
}

func toString(m map[string][]byte) map[string]string {
	r := map[string]string{}
	for k, v := range m {
		r[k] = string(v)
	}
	return r
}
