// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Run,
		Desc:     "Always fails",
		Contacts: []string{"vsavu@google.com", "tast-owners@google.com"},
		Attr:     []string{"disabled"},
	})
}

func Run(ctx context.Context, s *testing.State) {
	s.Run(ctx, "ok", func(ctx context.Context, s *testing.State) {
		s.Log("ok")
	})

	s.Run(ctx, "error", func(ctx context.Context, s *testing.State) {
		s.Error("Here's an error")
	})

	s.Run(ctx, "fatal", func(ctx context.Context, s *testing.State) {
		s.Fatal("Here's a fatal error")
	})

	s.Run(ctx, "still-ok", func(ctx context.Context, s *testing.State) {
		s.Log("Still ok")
	})

	s.Run(ctx, "l1", func(ctx context.Context, s *testing.State) {
		s.Log("Level 1")

		s.Run(ctx, "l2", func(ctx context.Context, s *testing.State) {
			s.Log("Level 2")
		})
	})
}
