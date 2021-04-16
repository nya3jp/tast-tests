// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpBioWash,
		Desc: "Validate that bio_wash is called when clobber tool is run",
		Contacts: []string{
			"josienordrum@chromium.org", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      4 * time.Minute,
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService"},
		Vars:         []string{"servo"},
	})
}

func FpBioWash(ctx context.Context, s *testing.State) {
	t, err := fingerprint.NewFirmwareTest(ctx, s.DUT(), s.RequiredVar("servo"), s.RPCHint(), s.OutDir(), true, true)
	if err != nil {
		s.Fatal("Failed to create new firmware test: ", err)
	}
	ctxForCleanup := ctx
	defer func() {
		if err := t.Close(ctxForCleanup); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}
	}()
	ctx, cancel := ctxutil.Shorten(ctx, fingerprint.TimeForCleanup)
	defer cancel()

	d := t.DUT()

	testing.ContextLog(ctx, "Initializing FW entropy")
	if err := fingerprint.AddEntropy(ctx, d, false); err != nil {
		s.Fatal("Failed to add FW entropy")
	}

	testing.ContextLog(ctx, "Validating rollback was reset")
	if err := fingerprint.CheckRollbackSetToInitialValue(ctx, d); err != nil {
		s.Fatal("Failed to validate rollback state: ", err)
	}
}
