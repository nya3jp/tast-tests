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
		Func: CrosDisksFilesystem,
		Desc: "Verify that cros-disks can mount supported filesystems correctly",
		Contacts: []string{
			"amistry@chromium.org",
			"benchan@chromium.org",
			"chromeos-files-app@google.com",
		},
		// This test is still under development. Do not promote to CQ.
		Attr: []string{"informational"},
	})
}

func CrosDisksFilesystem(ctx context.Context, s *testing.State) {
	crosdisks.RunFilesystemTests(ctx, s)
}
