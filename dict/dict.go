// Example:
//
// payload, err := dict.Unmarshal(b)
//
// if value, ok := payload.TryGetString("value"); ok {
//     print(value)
// }
//
// value := payload.GetDict("result").GetString("value", "default")
package dict

import (
	"encoding/json"
)

type Dict map[string]interface{}

// Decode JSON object to Dict class
func Unmarshal(b []byte) (*Dict, error) {
	payload := make(Dict)
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (d *Dict) TryGetString(name string) (string, bool) {
	switch (*d)[name].(type) {
	case string:
		return (*d)[name].(string), true
	}
	return "", false
}

func (d *Dict) TryGetNumber(name string) (float64, bool) {
	switch (*d)[name].(type) {
	case float64:
		return (*d)[name].(float64), true
	}
	return 0, false
}

func (d *Dict) GetDict(name string) *Dict {
	switch (*d)[name].(type) {
	case interface{}:
		i := Dict((*d)[name].(map[string]interface{}))
		return &i
	}
	return nil
}

func (d *Dict) GetArrayItem(name string, index int) *Dict {
	switch (*d)[name].(type) {
	case []interface{}:
		l := (*d)[name].([]interface{})
		if len(l) > index {
			i := Dict(l[index].(map[string]interface{}))
			return &i
		}
	}
	return nil
}

func (d *Dict) GetString(name string, def string) string {
	switch (*d)[name].(type) {
	case string:
		return (*d)[name].(string)
	}
	return def
}

func (d *Dict) GetFloat(name string, def float64) float64 {
	switch (*d)[name].(type) {
	case float64:
		return (*d)[name].(float64)
	}
	return def
}

func (d *Dict) GetUint8(name string, def uint8) uint8 {
	switch (*d)[name].(type) {
	case float64:
		return uint8((*d)[name].(float64))
	}
	return def
}

func (d *Dict) GetUint16(name string, def uint16) uint16 {
	switch (*d)[name].(type) {
	case float64:
		return uint16((*d)[name].(float64))
	}
	return def
}

func (d *Dict) GetUint32(name string, def uint32) uint32 {
	switch (*d)[name].(type) {
	case float64:
		return uint32((*d)[name].(float64))
	}
	return def
}

func (d *Dict) GetUint64(name string, def uint64) uint64 {
	switch (*d)[name].(type) {
	case float64:
		return uint64((*d)[name].(float64))
	}
	return def
}
