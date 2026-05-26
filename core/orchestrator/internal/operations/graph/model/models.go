// Package model holds the hand-written GraphQL model types + custom
// scalars. Everything in this file is forward-stable: gqlgen autobinds
// against this package, so changing a field here is a schema-breaking
// change and goes through the normal review.
//
// The generated_models.go sibling holds gqlgen's generated input / output
// types and is overwritten by every `gqlgen generate` run. Do not edit
// generated_models.go by hand.
package model

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/99designs/gqlgen/graphql"
	"github.com/shopspring/decimal"
)

// JSON is the catch-all scalar for free-form payloads (MCP envelopes,
// agent telemetry blobs, raw provider responses). The Go type is
// map[string]any so resolvers can mutate it without a type assertion.
type JSON map[string]any

// MarshalGQL implements graphql.Marshaler for JSON.
func (j JSON) MarshalGQL(w io.Writer) {
	if j == nil {
		_, _ = w.Write([]byte("null"))
		return
	}
	bts, err := json.Marshal(map[string]any(j))
	if err != nil {
		_, _ = w.Write([]byte("null"))
		return
	}
	_, _ = w.Write(bts)
}

// UnmarshalGQL implements graphql.Unmarshaler for JSON. We accept either
// a Go map (the default JSON path through gqlgen's HTTP transport) or
// any value json.Marshal can round-trip back into a map.
func (j *JSON) UnmarshalGQL(v any) error {
	if v == nil {
		*j = nil
		return nil
	}
	switch t := v.(type) {
	case map[string]any:
		*j = JSON(t)
		return nil
	case string:
		var out map[string]any
		if err := json.Unmarshal([]byte(t), &out); err != nil {
			return fmt.Errorf("JSON scalar: invalid JSON string: %w", err)
		}
		*j = JSON(out)
		return nil
	case []byte:
		var out map[string]any
		if err := json.Unmarshal(t, &out); err != nil {
			return fmt.Errorf("JSON scalar: invalid JSON bytes: %w", err)
		}
		*j = JSON(out)
		return nil
	default:
		bts, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("JSON scalar: unsupported type %T", v)
		}
		var out map[string]any
		if err := json.Unmarshal(bts, &out); err != nil {
			return fmt.Errorf("JSON scalar: %T cannot be coerced into JSON object", v)
		}
		*j = JSON(out)
		return nil
	}
}

// Decimal mirrors shopspring/decimal.Decimal but implements the gqlgen
// scalar interface. We use a named alias instead of embedding so the
// JSON struct tags stay clean and `Decimal{}.IsZero()` still works.
type Decimal decimal.Decimal

// AsDecimal returns the underlying shopspring decimal — handy for math.
func (d Decimal) AsDecimal() decimal.Decimal { return decimal.Decimal(d) }

// NewDecimal wraps a shopspring decimal into the scalar alias.
func NewDecimal(d decimal.Decimal) Decimal { return Decimal(d) }

// MarshalGQL emits the decimal as a JSON string ("1.23"). Numbers in JS
// lose precision past 15 digits so strings are the safe wire format.
func (d Decimal) MarshalGQL(w io.Writer) {
	_, _ = w.Write([]byte(strconv.Quote(decimal.Decimal(d).String())))
}

// UnmarshalGQL parses the decimal from a string or a numeric.
func (d *Decimal) UnmarshalGQL(v any) error {
	switch t := v.(type) {
	case string:
		dec, err := decimal.NewFromString(t)
		if err != nil {
			return fmt.Errorf("Decimal scalar: %w", err)
		}
		*d = Decimal(dec)
		return nil
	case float64:
		*d = Decimal(decimal.NewFromFloat(t))
		return nil
	case json.Number:
		dec, err := decimal.NewFromString(string(t))
		if err != nil {
			return fmt.Errorf("Decimal scalar: %w", err)
		}
		*d = Decimal(dec)
		return nil
	case int64:
		*d = Decimal(decimal.NewFromInt(t))
		return nil
	default:
		return fmt.Errorf("Decimal scalar: unsupported type %T", v)
	}
}

// Bytes is the binary scalar. We base64-encode on the wire so the JSON
// payload stays text-safe; resolvers receive a raw []byte.
type Bytes []byte

// MarshalGQL emits the bytes as a base64-encoded JSON string.
func (b Bytes) MarshalGQL(w io.Writer) {
	graphql.MarshalString(b64Encode(b)).MarshalGQL(w)
}

// UnmarshalGQL decodes a base64-encoded JSON string into bytes.
func (b *Bytes) UnmarshalGQL(v any) error {
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("Bytes scalar: expected string, got %T", v)
	}
	out, err := b64Decode(s)
	if err != nil {
		return fmt.Errorf("Bytes scalar: %w", err)
	}
	*b = out
	return nil
}
