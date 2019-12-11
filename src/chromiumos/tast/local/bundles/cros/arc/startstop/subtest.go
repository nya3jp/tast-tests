// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package startstop

import (
	"context"

	"chromiumos/tast/testing"
)

// Subtest defines a test runs in arc.StartStop.
// Each implementation of Subtest can use the following methods to check some
// conditions, or to record some values to be used in a method called later.
// Even if an error is reported in a method, another method will be called
// unless the arc.StartStop is not terminated. For example, even if s.Error()
// is called in PostStart(), PostStop() for the same Subtest instance may be
// called later.
// The invocation of PreStart, PostStart and PostStop is wrapped by
// testing.State.Run, so even if s.Fatal() or s.Fatalf() is called, other
// methods will be called, still.
type Subtest interface {
	// Name returns the name of this subtest.
	Name() string

	// PreStart is called before starting Chrome.
	PreStart(ctx context.Context, s *testing.State)

	// PostStart is called after starting ARC, i.e. called during the
	// ARC session.
	PostStart(ctx context.Context, s *testing.State)

	// PostStop is called after logout from Chrome.
	PostStop(ctx context.Context, s *testing.State)
}
