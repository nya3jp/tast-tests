// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LocalReboot,
		Desc:         "Triggers an intentional reboot",
		Contacts:     []string{"tast-owners@google.com"},
		BugComponent: "b:1034625",
	})
}

func LocalReboot(ctx context.Context, s *testing.State) {
	// DON'T TRY THIS AT HOME.
	// Local tests should not reboot the DUT. This test is exceptional because
	// this test checks the behavior of Tast itself when a local test wrongly
	// reboots the DUT.
	if err := testexec.CommandContext(ctx, "reboot").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("reboot failed: ", err)
	}

	// Wait until the test timeout is reached.
	<-ctx.Done()
}
