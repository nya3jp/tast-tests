// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/arc/minicontainer"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MiniContainerState,
		Desc: "Verifies ARC mini container starts right after Chrome OS shows the login screen",
		Contacts: []string{
			"yusukes@chromium.org", // Original author.
			"arc-core@google.com",
			"hidehiko@chromium.org", // Tast port.
		},
		Attr: []string{"group:mainline", "informational"},
		// TODO(crbug.com/952125): Consider to relax the SoftwareDeps.
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
	})
}

func MiniContainerState(ctx context.Context, s *testing.State) {
	minicontainer.RunTest(ctx, s)
}
