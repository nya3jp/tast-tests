// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DaemonsRestartStress,
		Desc: "Verifies that restarting hwsec daemons wouldn't cause problems",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@chromium.org",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      10 * time.Minute,
	})
}

// DaemonsRestartStress checks that restarting hwsec daemons wouldn't cause problems.
func DaemonsRestartStress(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	tpmManager := helper.TPMManagerClient()
	daemonController := helper.DaemonController()

	// Check that lockout shouldn't be in effect.
	info, err := tpmManager.GetDAInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get dictionary attack info: ", err)
	}
	if info.InEffect {
		s.Fatal("Lockout in effect before testing")
	}

	ctxForResumeDaemons := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	// Drop the DA reset permission.
	restorePermCall, err := helper.DropResetLockPermissions(ctx)
	if err != nil {
		s.Fatal("Failed to drop the DA reset permission: ", err)
	}
	defer func(ctx context.Context) {
		// Restore the DA reset permission.
		if err = restorePermCall(ctx); err != nil {
			s.Log("Failed to restore lockout permission: ", err)
		}
	}(ctxForResumeDaemons)

	tpmVer, err := helper.GetTPMVersion(ctx)
	if err != nil {
		s.Fatal("Failed to get TPM version: ", err)
	}

	// Restart TPM related daemons multiple times.
	for i := 0; i < 10; i++ {
		func() {
			if err := daemonController.Stop(ctx, hwsec.CryptohomeDaemon); err != nil {
				s.Fatal("Failed to stop cryptohomed: ", err)
			}
			defer func() {
				if err := daemonController.Start(ctx, hwsec.CryptohomeDaemon); err != nil {
					s.Fatal("Failed to start cryptohomed: ", err)
				}
			}()

			if err := daemonController.Stop(ctx, hwsec.AttestationDaemon); err != nil {
				s.Fatal("Failed to stop attestationd: ", err)
			}
			defer func() {
				if err := daemonController.Start(ctx, hwsec.AttestationDaemon); err != nil {
					s.Fatal("Failed to start attestationd: ", err)
				}
			}()

			if err := daemonController.Stop(ctx, hwsec.TPMManagerDaemon); err != nil {
				s.Fatal("Failed to stop tpm_managerd: ", err)
			}
			defer func() {
				if err := daemonController.Start(ctx, hwsec.TPMManagerDaemon); err != nil {
					s.Fatal("Failed to start tpm_managerd: ", err)
				}
			}()

			switch tpmVer {
			case "1.2":
				if daemonController.Restart(ctx, hwsec.TcsdDaemon); err != nil {
					s.Fatal("Failed to restart tcsd: ", err)
				}
			case "2.0":
				if daemonController.Restart(ctx, hwsec.TrunksDaemon); err != nil {
					s.Fatal("Failed to restart trunksd: ", err)
				}
			default:
				s.Fatal("Unknown TPM version: ")
			}
		}()
	}

	// Check counter didn't increase too much, and lockout shouldn't be in effect.
	newInfo, err := tpmManager.GetDAInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get dictionary attack info: ", err)
	}

	// The counter shouldn't constantly increase when we restart daemons multiple time.
	// But it's acceptable to increase a little bit.
	// For example: we need one quota to check the validity of empty password.
	if newInfo.Counter > info.Counter+1 {
		s.Fatalf("Unexpected counter increase, %d -> %d", info.Counter, newInfo.Counter)
	}
	if newInfo.InEffect {
		s.Fatal("Lockout in effect after testing")
	}
}
