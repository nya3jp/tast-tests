// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Reboot,
		Desc:     "Reboots the DUT in the middle of the run",
		Contacts: []string{"tast-owners@chromium.org"},
	})
}

func Reboot(ctx context.Context, s *testing.State) {
	testexec.CommandContext(ctx, "reboot").Run()
	// The reboot command returns quickly. Make sure that this test doesn't have time to pass.
	testing.Sleep(ctx, 20*time.Second)
}
