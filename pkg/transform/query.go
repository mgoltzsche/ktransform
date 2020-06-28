package transform

import (
	"context"
	"fmt"

	"github.com/itchyny/gojq"
)

func Query(ctx context.Context, input map[string]interface{}, query string) (interface{}, error) {
	q, err := gojq.Parse(query)
	if err != nil {
		return nil, err
	}
	iter := q.RunWithContext(ctx, input)
	v, ok := iter.Next()
	if !ok {
		return nil, fmt.Errorf("query did not return anything: %s", query)
	}
	if err, ok := v.(error); ok {
		return nil, fmt.Errorf("query %s: %w", query, err)
	}
	return v, nil
}
