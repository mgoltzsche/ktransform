package transform

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/yaml"
)

func InputMapFromStringMap(data map[string]string) map[string]interface{} {
	input := map[string]interface{}{}
	if data != nil {
		for k, v := range data {
			val := map[string]interface{}{}
			val["string"] = v
			val["object"] = parseYaml([]byte(v))
			input[k] = val
		}
	}
	return input
}

func InputMapFromBytesMap(data map[string][]byte) map[string]interface{} {
	input := map[string]interface{}{}
	if data != nil {
		for k, v := range data {
			val := map[string]interface{}{}
			val["string"] = string(v)
			val["object"] = parseYaml(v)
			input[k] = val
		}
	}
	return input
}

func BytesMapFromOutput(m map[string]interface{}) (map[string][]byte, error) {
	r := map[string][]byte{}
	for k, v := range m {
		switch c := v.(type) {
		case []byte:
			r[k] = c
		case string:
			r[k] = []byte(c)
		case nil:
			r[k] = []byte{}
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("marshal key %s: %w", k, err)
			}
			r[k] = b
		}
	}
	return r, nil
}

func StringMapFromOutput(m map[string]interface{}) (map[string]string, error) {
	r := map[string]string{}
	for k, v := range m {
		switch c := v.(type) {
		case []byte:
			r[k] = string(c)
		case string:
			r[k] = c
		case nil:
			r[k] = ""
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			r[k] = string(b)
		}
	}
	return r, nil
}

func parseYaml(data []byte) map[string]interface{} {
	m := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}
