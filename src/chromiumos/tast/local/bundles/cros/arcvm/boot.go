// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcvm

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Boot,
		Desc: "Checks that Android boots",
		// TODO(nya): Add a proper contact of ARC boot tests.
		Contacts: []string{"nya@chromium.org", "arc-eng@google.com"},
		Attr:     []string{"informational"},
		SoftwareDeps: []string{
			"android_all_vm", // Run on master-arc-dev, too.
			"chrome",
		},
		Timeout: 4 * time.Minute,
	})
}

func Boot(ctx context.Context, s *testing.State) {
	arc.Boot(ctx, s)
}
