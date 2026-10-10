package postgres

import "encoding/json"

func defaultJSONMarshal(v any) ([]byte, error)    { return json.Marshal(v) }
func defaultJSONUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }