// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package biod

import (
	"context"

	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Running,
		Desc: "Checks that biod is running on devices with fingerprint sensor",
		Contacts: []string{
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "group:fingerprint-cq"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
	})
}

// Running checks the biod job and fails if it isn't running or has a process
// in the zombie state.
func Running(ctx context.Context, s *testing.State) {
	if err := upstart.CheckJob(ctx, "biod"); err != nil {
		s.Fatal("Test failed: ", err)
	}
}
