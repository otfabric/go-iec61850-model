// SPDX-License-Identifier: MIT

package presence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/otfabric/go-iec61850-model/model"
)

// Limits of [ReadInstance].
const (
	// MaxInstanceBytes bounds the size of a JSON instance.
	MaxInstanceBytes = 4 << 20
	// MaxInstanceNames bounds the number of present names and values
	// of all levels together.
	MaxInstanceNames = 100000
)

// ReadInstance reads an instance in its JSON form:
//
//	{"class": "CSWI", "present": ["Beh", "Pos"],
//	 "context": {"ldNs": "IEC 61850-7-4:2007B", "facts": {"MOcond(1)": true}},
//	 "children": {"Pos": {"present": ["stVal", "q", "t", "ctlModel"],
//	                      "values": {"ctlModel": "direct-with-normal-security"}}}}
//
// Unknown fields, trailing data and input beyond [MaxInstanceBytes],
// [MaxInstanceNames] or [MaxDepth] levels fail with
// [model.ErrInvalidInstance] (and [model.ErrLimitExceeded] for the
// limits).
func ReadInstance(r io.Reader) (*Instance, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: nil reader", model.ErrInvalidInstance)
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxInstanceBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrInvalidInstance, err)
	}
	if len(data) > MaxInstanceBytes {
		return nil, fmt.Errorf("%w: %w: instance larger than %d bytes", model.ErrInvalidInstance, model.ErrLimitExceeded, MaxInstanceBytes)
	}
	if err := checkDepth(data); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var inst Instance
	if err := dec.Decode(&inst); err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrInvalidInstance, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("%w: trailing data after the instance", model.ErrInvalidInstance)
	}
	n := 0
	if err := inst.check(0, &n); err != nil {
		return nil, err
	}
	return &inst, nil
}

// checkDepth rejects JSON nested deeper than the instance format can
// be, before decoding: each instance level adds two (children, entry).
func checkDepth(data []byte) error {
	const maxNesting = 2*MaxDepth + 4
	depth, inString, escaped := 0, false, false
	for _, b := range data {
		switch {
		case inString:
			switch {
			case escaped:
				escaped = false
			case b == '\\':
				escaped = true
			case b == '"':
				inString = false
			}
		case b == '"':
			inString = true
		case b == '{' || b == '[':
			depth++
			if depth > maxNesting {
				return fmt.Errorf("%w: %w: instance nested deeper than %d levels", model.ErrInvalidInstance, model.ErrLimitExceeded, MaxDepth)
			}
		case b == '}' || b == ']':
			depth--
		}
	}
	return nil
}

func (inst *Instance) check(depth int, n *int) error {
	if depth > MaxDepth {
		return fmt.Errorf("%w: %w: instance nested deeper than %d levels", model.ErrInvalidInstance, model.ErrLimitExceeded, MaxDepth)
	}
	*n += len(inst.Present) + len(inst.Values) + len(inst.Context.Facts)
	if *n > MaxInstanceNames {
		return fmt.Errorf("%w: %w: more than %d names", model.ErrInvalidInstance, model.ErrLimitExceeded, MaxInstanceNames)
	}
	for name, c := range inst.Children {
		if name == "" {
			return fmt.Errorf("%w: child with an empty name", model.ErrInvalidInstance)
		}
		if c == nil {
			return fmt.Errorf("%w: child %q is null", model.ErrInvalidInstance, name)
		}
		if err := c.check(depth+1, n); err != nil {
			return err
		}
	}
	return nil
}
