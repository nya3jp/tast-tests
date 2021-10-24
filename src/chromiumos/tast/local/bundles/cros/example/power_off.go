// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PowerOff,
		Desc:     "Shut down a DUT to simulate a DUT losing connectivity",
		Contacts: []string{"seewaifu@google.com"},
	})
}

// PowerOff turns off power to simulate a DUT losing connectivity.
func PowerOff(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "/sbin/shutdown", "-h", "now")
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to shutdown DUT: %v -- %v", err, out)
	}
}
