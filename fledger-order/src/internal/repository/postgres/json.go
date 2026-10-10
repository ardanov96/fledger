package postgres

import (
	"encoding/json"
)

// jsonMarshal and jsonUnmarshal are tiny indirection helpers so the test
// file does not need to import encoding/json in every repo.
func jsonMarshal(v any) ([]byte, error)    { return json.Marshal(v) }
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }