// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TPMManagerPerf,
		Desc: "TPM manager performance test",
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

const (
	notOwned = "owned: false"
)

// waitUntilTPMManagerReady is a helper function to wait until cryptohome initialized.
func waitUntilTPMManagerReady(ctx context.Context, tpmManagerUtil *hwsec.UtilityTPMManagerBinary) error {
	return testing.Poll(ctx, func(context.Context) error {
		status, err := tpmManagerUtil.Status(ctx)
		if err != nil {
			return errors.Wrap(err, "can't get TPM status")
		}
		if !strings.Contains(status, notOwned) {
			return errors.New("TPM isn't ready to be owned")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: time.Second,
	})
}

// TPMManagerPerf do the performance test for tpm_manager.
func TPMManagerPerf(ctx context.Context, s *testing.State) {
	r, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("Failed to create new command runner: ", err)
	}

	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}

	helper, err := hwsecremote.NewHelper(utility, r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMIsResetAndPowerwash(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	tpmManagerUtil, err := hwsec.NewUtilityTPMManagerBinary(r)
	if err != nil {
		s.Fatal("Failed to create UtilityTPMManagerBinary: ", err)
	}

	err = waitUntilTPMManagerReady(ctx, tpmManagerUtil)
	if err != nil {
		s.Fatal("Failed to wait tpm_manager ready: ", err)
	}

	takeOwnershipStart := time.Now()
	tpmManagerUtil.TakeOwnership(ctx)
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
