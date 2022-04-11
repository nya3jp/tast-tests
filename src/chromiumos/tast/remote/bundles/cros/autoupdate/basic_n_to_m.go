// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"

	"chromiumos/tast/remote/bundles/cros/autoupdate/autoupdatelib"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BasicNToM,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Example test for updating to an older version using Nebraska and test images",
		Contacts: []string{
			"gabormagda@google.com", // Test author
		},
		Attr:         []string{}, // Manual execution only.
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.UpdateService",
		},
		Timeout: autoupdatelib.TotalTestTime,
	})
}

func BasicNToM(ctx context.Context, s *testing.State) {
	autoupdatelib.NToMTest(ctx, s, &autoupdatelib.Operations{}, 3 /*deltaM*/)
}
