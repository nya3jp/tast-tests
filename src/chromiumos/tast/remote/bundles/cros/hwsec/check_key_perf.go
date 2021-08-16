// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckKeyPerf,
		Desc: "Performance for CheckKey operation",
		Contacts: []string{
			"dlunev@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"hwsec_destructive_crosbolt_perbuild", "group:hwsec_destructive_crosbolt"},
		SoftwareDeps: []string{"tpm", "reboot"},
		Vars:         []string{"hwsec.CheckKeyPerf.iterations"},
	})
}

func CheckKeyPerf(ctx context.Context, s *testing.State) {
	// Setup helper functions
	r := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility := helper.CryptohomeClient()

	// Reset TPM
	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	// Create and Mount vault.
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user: ", err)
	}

	// Cleanup upon finishing
	defer func() {
		if _, err := utility.Unmount(ctx, util.FirstUsername); err != nil {
			s.Error("Failed to unmount vault: ", err)
		}
		if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
			s.Fatal("Failed to remove vault: ", err)
		}
	}()

	// Get iterations count from the variable or default it.
	iterations := int64(50)
	if val, ok := s.Var("hwsec.CheckKeyPerf.iterations"); ok {
		var err error
		iterations, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			s.Fatal("Unparsable iterations variable: ", err)
		}
	}

	value := perf.NewValues()

	// Run |iterations| times CheckKeyEx.
	for i := int64(0); i < iterations; i++ {
		startTs := time.Now()
		result, err := utility.CheckVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword))
		duration := time.Now().Sub(startTs)
		if err != nil {
			s.Fatal("Call to CheckKeyEx with the correct username and password resulted in an error: ", err)
		}
		if !result {
			s.Fatal("Failed to CheckKeyEx() with the correct username and password: ", err)
		}
		value.Append(perf.Metric{
			Name:      "check_key_ex_duration",
			Unit:      "us",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, float64(duration.Microseconds()))
	}

	if err := value.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf-results: ", err)
	}
}
