// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/platform/verity"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DMVerity,
		Desc:     "Verify dm-verity reports IO errors on bad data",
		Contacts: []string{"hidehiko@chromium.org"},
		Attr:     []string{"group:mainline"},
		Timeout:  4 * time.Minute,
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"dmverity_stable"},
			},
			{
				Name:              "unstable_kernel",
				ExtraSoftwareDeps: []string{"dmverity_unstable"},
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

func DMVerity(ctx context.Context, s *testing.State) {
	verity.RunTests(ctx, s)
}
