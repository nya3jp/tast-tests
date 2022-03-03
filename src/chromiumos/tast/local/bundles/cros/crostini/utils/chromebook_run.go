// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"

	"chromiumos/tast/testing"
)

// RunOrFatal refer to multi_display.go
func RunOrFatal(ctx context.Context, s *testing.State, name string, body func(context.Context, *testing.State) error) bool {
	return s.Run(ctx, name, func(ctx context.Context, s *testing.State) {
		if err := body(ctx, s); err != nil {
			s.Fatal("subtest failed: ", err)
		}
	})
}
