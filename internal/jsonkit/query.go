package jsonkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/itchyny/gojq"
)

// QueryLimit bounds the results of one query.
const QueryLimit = 1000

// QueryResult is what a jq filter gave: each output as JSON, indented.
type QueryResult struct {
	Results   []string `json:"results"`
	Truncated bool     `json:"truncated,omitempty"` // stopped at QueryLimit
}

// Query runs a jq filter over v (gojq; no environment, no input or
// output). jq's objects have no key order, so results list keys sorted,
// as jq -S does.
func Query(ctx context.Context, v *Value, expr, indent string) (QueryResult, error) {
	parsed, err := gojq.Parse(expr)
	if err != nil {
		var pe *gojq.ParseError
		if errors.As(err, &pe) {
			return QueryResult{}, fmt.Errorf("the filter has an error at character %d (%q): %s", pe.Offset+1, pe.Token, pe.Error())
		}
		return QueryResult{}, err
	}
	code, err := gojq.Compile(parsed, gojq.WithEnvironLoader(func() []string { return nil }))
	if err != nil {
		return QueryResult{}, err
	}
	out := QueryResult{Results: []string{}}
	iter := code.RunWithContext(ctx, toAny(v))
	for {
		r, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := r.(error); ok {
			var halt *gojq.HaltError
			if errors.As(err, &halt) && halt.Value() == nil {
				break
			}
			return out, fmt.Errorf("jq: %w", err)
		}
		if len(out.Results) == QueryLimit {
			out.Truncated = true
			break
		}
		b, err := json.Marshal(r)
		if err != nil {
			return out, err
		}
		pv, _, err := Parse(string(b))
		if err != nil {
			return out, err
		}
		out.Results = append(out.Results, Encode(pv, indent))
	}
	return out, nil
}

// toAny converts v to the types gojq works with: integers that fit stay
// exact (int or *big.Int), other numbers become float64.
func toAny(v *Value) any {
	switch v.Kind {
	case Bool:
		return v.Bool
	case Number:
		if isInteger(v.Num) {
			if n, err := strconv.Atoi(v.Num); err == nil {
				return n
			}
			if n, ok := new(big.Int).SetString(v.Num, 10); ok {
				return n
			}
		}
		f, _ := strconv.ParseFloat(v.Num, 64)
		return f
	case String:
		return v.Str
	case Array:
		out := make([]any, len(v.Items))
		for i, it := range v.Items {
			out[i] = toAny(it)
		}
		return out
	case Object:
		out := make(map[string]any, len(v.Members))
		for _, m := range v.Members {
			out[m.Key] = toAny(m.Value)
		}
		return out
	}
	return nil
}
