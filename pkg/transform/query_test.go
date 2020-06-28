package transform

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuery(t *testing.T) {
	num := 7
	str := "some value"
	strBase64 := base64.StdEncoding.EncodeToString([]byte(str))
	obj := map[string]interface{}{"num": num, "str": strBase64, "obj": map[interface{}]interface{}{"unsupported": num}}
	input := map[string]interface{}{"input": obj}
	for _, c := range []struct {
		name     string
		query    string
		input    map[string]interface{}
		expected interface{}
		valid    bool
	}{
		{"number", ".input.num", input, num, true},
		{"object", ".input", input, obj, true},
		{"string", ".input.str", input, strBase64, true},
		{"empty query", "", input, input, true},
		{"navigate over nil", ".input.nonexisting.obj", input, nil, true},
		{"invalid query", "invalid", input, nil, false},
		{"nil input", ".input.num", nil, nil, true},
		{"invalid input", ".input.obj.unsupported", input, nil, false},
		{"base64d", ".input.str | @base64d", input, str, true},
	} {
		testQuery(t, c.name, c.input, c.query, c.expected, c.valid)
	}
}

func testQuery(t *testing.T, name string, input map[string]interface{}, query string, expected interface{}, valid bool) {
	t.Run(name, func(t *testing.T) {
		output, err := Query(context.Background(), input, query)
		if valid {
			require.NoError(t, err, "query %s", query)
		} else {
			require.Error(t, err, "query %q should fail", query)
		}
		require.Equal(t, expected, output, "query %s", query)
	})
}
