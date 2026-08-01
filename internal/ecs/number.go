package ecs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

type Number struct {
	value float64
	valid bool
}

func (n *Number) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*n = Number{}
		return nil
	}
	var raw any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	var text string
	switch value := raw.(type) {
	case json.Number:
		text = value.String()
	case string:
		text = value
	default:
		return fmt.Errorf("expected number or numeric string")
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return fmt.Errorf("invalid finite number %q", text)
	}
	n.value = parsed
	n.valid = true
	return nil
}

func (n Number) Required(field string) (float64, error) {
	if !n.valid {
		return 0, fmt.Errorf("%s is required", field)
	}
	return n.value, nil
}

func (n Number) Optional() *float64 {
	if !n.valid {
		return nil
	}
	value := n.value
	return &value
}
