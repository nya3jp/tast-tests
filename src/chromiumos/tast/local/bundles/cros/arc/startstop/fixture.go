// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package startstop

import (
	"context"

	"chromiumos/tast/testing"
)

// Fixture is the test fixture used in the arc.StartStop test.
// Each implementation of Fixture can use the following hooks to check some
// conditions, or it may use the hooks to track some status to use the value
// in a later hook.
// Note that s.Fatal() will terminates the arc.StartStop() test, so it could
// cause that other Fixture tests does not run. In most cases, s.Error() then
// return may be expected.
type Fixture interface {
	// PreStart is called before starting Chrome.
	PreStart(ctx context.Context, s *testing.State)

	// PostStart is called after starting ARC, i.e. called during the
	// ARC runs.
	PostStart(ctx context.Context, s *testing.State)

	// PostStop is called after logout from Chrome.
	PostStop(ctx context.Context, s *testing.State)
}
