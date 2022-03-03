package utils

import (
	"context"

	"chromiumos/tast/testing"
)

// refer to multi_display.go
// runOrFatal runs body as subtest, then invokes s.Fatal if it returns an error
func RunOrFatal(ctx context.Context, s *testing.State, name string, body func(context.Context, *testing.State) error) bool {
	return s.Run(ctx, name, func(ctx context.Context, s *testing.State) {
		if err := body(ctx, s); err != nil {
			s.Fatal("subtest failed: ", err)
		}
	})
}
