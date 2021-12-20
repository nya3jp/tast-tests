// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/bundles/cros/platform/crosdisks"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosDisks,
		Desc: "Verifies CrosDisks' D-Bus API works",
		Contacts: []string{
			"chromeos-files-syd@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func CrosDisks(ctx context.Context, s *testing.State) {
	// Run series of tests. Please see crosdisks package for details.
	crosdisks.RunBasicTests(ctx, s)
}
