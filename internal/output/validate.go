// Package output writes the per-capture JSON plan and SVG rendering.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"roomscan/schema"
)

var (
	compileOnce sync.Once
	compiled    *jsonschema.Schema
	compileErr  error
)

func planSchema() (*jsonschema.Schema, error) {
	compileOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema.PlanJSON))
		if err != nil {
			compileErr = fmt.Errorf("parse plan schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		const url = "plan.schema.json"
		if err := c.AddResource(url, doc); err != nil {
			compileErr = err
			return
		}
		compiled, compileErr = c.Compile(url)
	})
	return compiled, compileErr
}

// Validate checks a JSON-encoded plan against schema/plan.schema.json.
func Validate(planJSON []byte) error {
	s, err := planSchema()
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(planJSON))
	if err != nil {
		return fmt.Errorf("plan is not valid JSON: %w", err)
	}
	return s.Validate(inst)
}

// ValidateValue marshals v and validates it.
func ValidateValue(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return Validate(b)
}
