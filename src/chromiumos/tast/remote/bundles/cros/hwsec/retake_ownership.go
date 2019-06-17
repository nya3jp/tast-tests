// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RetakeOwnership,
		Desc: "Verifies that the TPM ownership can be cleared and taken",
		Contacts: []string{
			"cylai@chromium.org", // Nobody
		},
		SoftwareDeps: []string{"reboot", "tpm"},
		Attr:         []string{"informational"},
	})
}

func RetakeOwnership(ctx context.Context, s *testing.State) {
	r, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
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
	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	if result, err := utility.IsPreparedForEnrollment(ctx); err != nil {
		s.Fatal("Cannot check if enrollment preparation is reset")
	} else if result {
		s.Fatal("Enrollment preparation is not reset after clearing ownership")
	}
	s.Log("Enrolling with TPM not ready")
	if _, err := utility.CreateEnrollRequest(ctx, hwsec.DefaultPCA); err == nil {
		s.Fatal("Enrollment should not happen w/o getting prepared")
	}

	s.Log("Start taking ownership")
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure ownership: ", err)
	}
	s.Log("Ownership is taken")

	// Checks owner password and attestation DB is still there after reboot,
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Fatal("Failed to prepare for enrollment: ", err)
	}
	s.Log("Attestation prepared")
	passwd, err := utility.GetOwnerPassword(ctx)
	if err != nil {
		s.Fatal("Failed to get owner password")
	} else if len(passwd) != 20 {
		s.Fatal("Ill-formed owner password")
	}
	// Verifies the owner password and attestatin DB is preserved.
	// Using sha256 might be overkill, but it doesn't really impact the testing,
	checksumOutput, err := r.Run(ctx, "sha256sum", hwsec.AttestationDBPath)
	if err != nil {
		s.Fatal("Error calculating checksum for attestation database")
	}
	if err := helper.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot")
	}
	if checksumOutput2, err := r.Run(ctx, "sha256sum", hwsec.AttestationDBPath); err != nil {
		s.Fatal("Error calculating checksum for attestation database")
	} else if !bytes.Equal(checksumOutput, checksumOutput2) {
		s.Fatal("Inconsistent checksum after reboot")
	}
	if passwd2, err := utility.GetOwnerPassword(ctx); err != nil {
		s.Fatal("Failed to get owner password")
	} else if passwd != passwd2 {
		s.Fatal("Inconsistent owner password after reboot")
	}

	s.Log("Clearing owner password")
	lastTime, err := r.Run(ctx, "stat", "-c", "%y", "/var/lib/tpm_manager/local_tpm_data")
	if err != nil {
		s.Log("Error calling stat; the polling operation will check the tpm password in every loop")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// This hacky logic watches the file modification of the persistent tpm status for both
		// monolithic and distributed models.
		// Ignores error here; if it's because file doesn't exist we assume the local data has changed.
		if err := utility.ClearOwnerPassword(ctx); err != nil {
			return err
		}
		newTime, err := r.Run(ctx, "stat", "-c", "%y", "/var/lib/tpm_manager/local_tpm_data")
		if err == nil && bytes.Equal(lastTime, newTime) {
			return errors.New("no local data change")
		}
		lastTime = newTime
		// For now, restarting cryptohome is necessary because we still use cryptohome binary.
		dCtrl := hwsec.NewDaemonController(r)
		if err := dCtrl.RestartCryptohome(ctx); err != nil {
			return err
		}
		if passwd, err := utility.GetOwnerPassword(ctx); err != nil {
			return err
		} else if len(passwd) != 0 {
			return errors.New("Still have password")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute}); err != nil {
		s.Fatal("Failed to wait for owner password to be cleared: ", err)
	}
}
