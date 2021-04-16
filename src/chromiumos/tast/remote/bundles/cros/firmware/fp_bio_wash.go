// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpBioWash,
		Desc: "Validate bio_wash behavior",
		Contacts: []string{
			"josienordrum@chromium.org", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      4 * time.Minute,
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService", dutfs.ServiceName},
		Vars:         []string{"servo"},
	})
}

func FpBioWash(ctx context.Context, s *testing.State) {
	t, err := fingerprint.NewFirmwareTest(ctx, s.DUT(), s.RequiredVar("servo"), s.RPCHint(), s.OutDir(), true, true, true)
	if err != nil {
		s.Fatal("Failed to create new firmware test: ", err)
	}
	ctxForCleanup := ctx
	defer func() {
		if err := t.Close(ctxForCleanup); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}
	}()
	ctx, cancel := ctxutil.Shorten(ctx, t.CleanupTime())
	defer cancel()

	d := t.DUT()

	testing.ContextLog(ctx, "Validating NewFirmwareTest initializes rollback correctly")
	if err := fingerprint.CheckRollbackState(ctx, d, fingerprint.RollbackState{
		BlockID: 0, MinVersion: 0, RWVersion: 0}); err != nil {
		s.Fatal("Unexpected inital rollback state: ", err)
	}

	testing.ContextLog(ctx, "Calling bio_wash with factory_init")
	if err := fingerprint.BioWash(ctx, d, true); err != nil {
		s.Fatal("Failed to call bio_wash with factory_init")
	}

	testing.ContextLog(ctx, "Validating rollback block ID is 1")
	if err := fingerprint.CheckRollbackState(ctx, d, fingerprint.RollbackState{
		BlockID: 1, MinVersion: 0, RWVersion: 0}); err != nil {
		s.Fatal("Unexpected rollback state: ", err)
	}

	testing.ContextLog(ctx, "Calling bio_wash")
	if err := fingerprint.BioWash(ctx, d, false); err != nil {
		s.Fatal("Failed to call bio_wash")
	}

	testing.ContextLog(ctx, "Validating Block ID increases by 2, but nothing else")
	if err := fingerprint.CheckRollbackState(ctx, d, fingerprint.RollbackState{
		BlockID: 3, MinVersion: 0, RWVersion: 0}); err != nil {
		s.Fatal("Unexpected rollback state: ", err)
	}
}
