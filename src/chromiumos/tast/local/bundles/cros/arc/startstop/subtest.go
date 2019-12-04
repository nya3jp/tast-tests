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
// Note that s.Fatal() will terminate the arc.StartStop test, so it could
// prevent other Subtest instances from running. In most cases,
// "s.Error() then return" may be expected.
type Subtest interface {
	// PreStart is called before starting Chrome.
	PreStart(ctx context.Context, s *testing.State)

	// PostStart is called after starting ARC, i.e. called during the
	// ARC session.
	PostStart(ctx context.Context, s *testing.State)

	// PostStop is called after logout from Chrome.
	PostStop(ctx context.Context, s *testing.State)
}
