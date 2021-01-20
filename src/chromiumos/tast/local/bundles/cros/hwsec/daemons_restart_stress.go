// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
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

func checkAndRestartDaemons(ctx context.Context) (lastErr error) {
	if err := cryptohome.CheckService(ctx); err != nil {
		return errors.Wrap(err, "Cryptohome D-Bus service didn't come back")
	}
	resumeCallback, err := hwseclocal.ResetTPMDaemons(ctx)
	defer func() {
		lastErr = resumeCallback()
	}()
	if err != nil {
		return err
	}
	return lastErr
}

// DaemonsRestartStress checks that restarting hwsec daemons wouldn't cause problems.
func DaemonsRestartStress(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}
	tpmManagerUtil, err := hwsec.NewUtilityTpmManagerBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityTpmManagerBinary: ", err)
	}

	// Check that lockout shouldn't be in effect.
	info, err := tpmManagerUtil.GetDAInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get dictionary attack info: ", err)
	}
	if info.InEffect {
		s.Fatal("Lockout in effect before testing")
	}

	ctxForResumeDaemons := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	var restorePermCall func() error

	// Drop the reset lock permission.
	func() {
		resumeCallback, err := hwseclocal.ResetTPMDaemons(ctx)
		defer func(ctx context.Context) {
			if err := resumeCallback(ctx); err != nil {
				s.Log("Failed to resume TPM daemons: ", err)
			}
		}(ctxForResumeDaemons)
		if err != nil {
			s.Fatal("Failed to reset TPM daemon: ", err)
		}

		restorePermCall, err = tpmManagerUtil.DropResetLockPermissions(ctx)
		if err != nil {
			s.Fatal("Failed to drop lockout permission: ", err)
		}
	}()

	defer func(ctx context.Context) {
		resumeCallback, err := hwseclocal.ResetTPMDaemons(ctx)
		defer func() {
			// Resume all TPM daemons after test finish.
			if err := resumeCallback(ctx); err != nil {
				s.Log("Failed to resume TPM daemons: ", err)
			}
			// Reset the DA counter after test finish.
			if _, err := tpmManagerUtil.ResetDALock(ctx); err != nil {
				s.Fatal("Failed to reset DA lock: ", err)
			}
		}()
		if err != nil {
			s.Log("Failed to reset TPM daemon: ", err)
		}

		// Restore the reset lock permission.
		if err = restorePermCall(ctx); err != nil {
			s.Log("Failed to restore lockout permission: ", err)
		}
	}(ctxForResumeDaemons)

	// Restart TPM related daemons multiple times.
	for i := 0; i < 8; i++ {
		if err := checkAndRestartDaemons(ctx); err != nil {
			s.Fatal("Failed to check and restart TPM Daemons: ", err)
		}
	}

	// Check counter didn't increase too much, and lockout shouldn't be in effect.
	newInfo, err := tpmManagerUtil.GetDAInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get dictionary attack info: ", err)
	}
	if newInfo.Counter > info.Counter+2 {
		s.Fatalf("Unexpected counter increase, %d -> %d", info.Counter, newInfo.Counter)
	}
	if newInfo.InEffect {
		s.Fatal("Lockout in effect after testing")
	}
}
