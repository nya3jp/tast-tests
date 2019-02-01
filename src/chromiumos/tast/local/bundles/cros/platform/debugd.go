// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/bundles/cros/platform/debugd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Debugd,
		Desc: "Verifies Debugd's D-Bus API works",
		Attr: []string{"informational"},
	})
}

func Debugd(ctx context.Context, s *testing.State) {
	// Run series of tests. Please see crosdisks package for details.
	debugd.RunTests(ctx, s)
}
