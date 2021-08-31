// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeMountPerf,
		Desc: "Performance for cryptohome mount operation",
		Contacts: []string{
			"yich@google.com", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"hwsec_destructive_crosbolt_perbuild", "group:hwsec_destructive_crosbolt"},
		SoftwareDeps: []string{"tpm", "reboot"},
		Vars: []string{
			"hwsec.CryptohomeMountPerf.normalMountIterations",
			"hwsec.CryptohomeMountPerf.rebootMountIterations",
		},
		ServiceDeps: []string{
			"tast.cros.hwsec.AttestationDBusService",
		},
		Timeout: 6 * time.Minute,
	})
}

// CryptohomeMountPerf collects the performance for cryptohome mount operation.
func CryptohomeMountPerf(ctx context.Context, s *testing.State) {
	// Setup helper functions
	cmdRunner := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewFullHelper(cmdRunner, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	cryptohome := helper.CryptohomeClient()
	daemonController := helper.DaemonController()

	chaps, err := pkcs11.NewChaps(ctx, cmdRunner, helper.CryptohomeClient())
	if err != nil {
		s.Fatal("Failed to create chaps client: ", err)
	}

	// Reset the TPM.
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}

	// Ensure TPM is ready.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM ready: ", err)
	}

	// Ensure attestation finished the initialization.
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Fatal("Failed to prepare for enrollment: ", err)
	}

	// ensureChapsSlotsInitialized ensures chaps is initialized.
	ensureChapsSlotsInitialized := func(ctx context.Context, chaps *pkcs11.Chaps) error {
		return testing.Poll(ctx, func(context.Context) error {
			slots, err := chaps.ListSlots(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to list chaps slots")
			}
			if len(slots) < 1 {
				return errors.Wrap(err, "chaps initialization hasn't finished")
			}
			return nil
		}, &testing.PollOptions{
			Timeout:  30 * time.Second,
			Interval: time.Second,
		})
	}

	// Ensure chaps finished the initialization.
	if err := ensureChapsSlotsInitialized(ctx, chaps); err != nil {
		s.Fatal("Failed to ensure chaps slots: ", err)
	}

	// Create and Mount vault.
	startTs := time.Now()
	if err := cryptohome.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user: ", err)
	}
	createMountDuration := time.Now().Sub(startTs)

	// Cleanup upon finishing.
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*5)
	defer cancel()
	defer func(ctx context.Context) {
		if _, err := cryptohome.Unmount(ctx, util.FirstUsername); err != nil {
			s.Error("Failed to unmount vault: ", err)
		}
		if _, err := cryptohome.RemoveVault(ctx, util.FirstUsername); err != nil {
			s.Fatal("Failed to remove vault: ", err)
		}
	}(ctxForCleanup)

	// Unmount before we start the test.
	if _, err := cryptohome.Unmount(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to unmount vault: ", err)
	}

	valueOrDefaultForVar := func(s *testing.State, key string, defaultValue int) int {
		if val, ok := s.Var(key); ok {
			var err error
			converted, err := strconv.Atoi(val)
			if err != nil {
				s.Fatalf("Unable to parse %v variable: %v", key, err)
			}
			return converted
		}
		return defaultValue
	}

	// Get iterations count from the variable or default it.
	normalMountIterations := valueOrDefaultForVar(s, "hwsec.CryptohomeMountPerf.normalMountIterations", 10)

	rebootMountIterations := valueOrDefaultForVar(s, "hwsec.CryptohomeMountPerf.rebootMountIterations", 3)

	value := perf.NewValues()

	// Report the duration of creating the vault.
	value.Append(perf.Metric{
		Name:      "create_mount_duration",
		Unit:      "us",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, float64(createMountDuration.Microseconds()))

	// Run normalMountIterations times MountVault.
	for i := 0; i < normalMountIterations; i++ {
		startTs := time.Now()
		err := cryptohome.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, false, hwsec.NewVaultConfig())
		duration := time.Now().Sub(startTs)

		if err != nil {
			s.Fatal("Failed to mount vault: ", err)
		}

		value.Append(perf.Metric{
			Name:      "normal_mount_duration",
			Unit:      "us",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, float64(duration.Microseconds()))

		startTs = time.Now()
		_, err = cryptohome.Unmount(ctx, util.FirstUsername)
		duration = time.Now().Sub(startTs)

		if err != nil {
			s.Fatal("Failed to unmount vault: ", err)
		}

		value.Append(perf.Metric{
			Name:      "normal_unmount_duration",
			Unit:      "us",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, float64(duration.Microseconds()))
	}

	// Run rebootMountIterations times MountVault after reboot.
	for i := 0; i < rebootMountIterations; i++ {
		if err := helper.Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot: ", err)
		}
		if err := daemonController.WaitForAllDBusServices(ctx); err != nil {
			s.Fatal("Failed to wait for hwsec D-Bus services to be ready: ", err)
		}

		// Ensure attestation finished the initialization.
		if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
			s.Fatal("Failed to prepare for enrollment: ", err)
		}
		// Ensure chaps finished the initialization.
		if err := ensureChapsSlotsInitialized(ctx, chaps); err != nil {
			s.Fatal("Failed to ensure chaps slots: ", err)
		}

		startTs := time.Now()
		err := cryptohome.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, false, hwsec.NewVaultConfig())
		duration := time.Now().Sub(startTs)
		if err != nil {
			s.Fatal("Failed to mount vault: ", err)
		}

		value.Append(perf.Metric{
			Name:      "mount_after_reboot_duration",
			Unit:      "us",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, float64(duration.Microseconds()))

		startTs = time.Now()
		_, err = cryptohome.Unmount(ctx, util.FirstUsername)
		duration = time.Now().Sub(startTs)

		if err != nil {
			s.Fatal("Failed to unmount vault: ", err)
		}

		value.Append(perf.Metric{
			Name:      "unmount_after_reboot_duration",
			Unit:      "us",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, float64(duration.Microseconds()))
	}

	if err := value.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf-results: ", err)
	}
}
