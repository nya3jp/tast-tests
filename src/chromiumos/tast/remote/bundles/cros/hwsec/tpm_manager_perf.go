// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/perf"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TpmManagerPerf,
		Desc: "Tpm manager performance test",
		Attr: []string{
			"hwsec_destructive_crosbolt_perbuild",
			"group:hwsec_destructive_crosbolt",
		},
		Contacts: []string{
			"yich@google.com",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"reboot", "tpm"},
	})
}

// TpmManagerPerf do the performance test for tpm_manager.
func TpmManagerPerf(ctx context.Context, s *testing.State) {
	r := hwsecremote.NewCmdRunner(s.DUT())

	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	tpmManager := helper.TPMManagerClient()

	dCtrl := helper.DaemonController()
	dCtrl.Ensure(ctx, hwsec.TPMManagerDaemon)

	takeOwnershipStart := time.Now()
	tpmManager.TakeOwnership(ctx)
	takeOwnershipElapsed := time.Since(takeOwnershipStart)

	s.Log("time to take tpm ownership: ", takeOwnershipElapsed.Seconds())

	// Record the perf measurements.
	value := perf.NewValues()

	value.Set(perf.Metric{
		Name:      "take_ownership",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, takeOwnershipElapsed.Seconds())

	value.Save(s.OutDir())
}
