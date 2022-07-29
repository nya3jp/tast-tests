// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/rollback"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnterpriseRollbackInPlace,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check the enterprise rollback data restore mechanism while faking a rollback on one image",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"crisguerrero@chromium.org",
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.hwsec.OwnershipService",
			"tast.cros.autoupdate.RollbackService",
		},
		Timeout: 10 * time.Minute,
	})
}

// EnterpriseRollbackInPlace does not expect to use enrollment so any
// functionality that depend on the enrollment of the device should be not be
// added to this test.
func EnterpriseRollbackInPlace(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		if err := rollback.ClearRollbackAndSystemData(ctx, s.DUT(), s.RPCHint()); err != nil {
			s.Error("Failed to clean rollback data after test: ", err)
		}
	}(cleanupCtx)

	if err := rollback.SimulatePowerwash(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to simulate powerwash before test: ", err)
	}

	networksInfo, err := rollback.ConfigureNetworks(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to configure networks: ", err)
	}

	sensitive, err := rollback.SaveRollbackData(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to save rollback data: ", err)
	}

	// Ineffective reset is ok here as the device steps through oobe automatically
	s.Log("Simulating powerwash and rebooting the DUT to fake rollback")
	if err := rollback.SimulatePowerwashAndReboot(ctx, s.DUT()); err != nil && !errors.Is(err, hwsec.ErrIneffectiveReset) {
		s.Fatal("Failed to simulate powerwash and reboot to fake an enterprise rollback: ", err)
	}

	if err := rollback.VerifyRollbackData(ctx, s.DUT(), s.RPCHint(), networksInfo, sensitive); err != nil {
		s.Fatal("Failed to verify rollback: ", err)
	}
}
