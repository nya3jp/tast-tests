// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HWDeps,
		Desc:         "Sanity check and demonstration of hardware deps feature",
		Contacts:     []string{"tast-owners@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Model("eve")),
	})
}

func HWDeps(ctx context.Context, s *testing.State) {
	// No errors means the test passed.
	// This test should run only on eve models.
}
