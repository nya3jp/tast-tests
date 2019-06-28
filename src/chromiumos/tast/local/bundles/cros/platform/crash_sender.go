// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/bundles/cros/platform/crashsender"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrashSender,
		Desc: "Verify crash sender works",
		Contacts: []string{
			"benchan@chromium.org",    // Autotest author
			"jkardatzke@chromium.org", // Autotest author
			"vapier@chromium.org",     // Autotest author
			"chavey@chromium.org",     // Migrated autotest to tast
		},
		Attr: []string{"informational"},
		Data: []string{
			crashsender.MockMetricsOffPolicyFile,
			crashsender.MockMetricsOnPolicyFile,
			crashsender.MockMetricsOwnerKeyFile,
		},
	})
}

func CrashSender(ctx context.Context, s *testing.State) {
	crashsender.RunTests(ctx, s)
}
