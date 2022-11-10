// Copyright 2020 The ChromiumOS Authors
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

// retakeOwnershipLatePreparationWithAuthAPIParam contains the test parameters which are different
// between the types of backing store.
type retakeOwnershipLatePreparationWithAuthAPIParam struct {
	// Specifies whether to use user secret stash.
	useUserSecretStash bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         RetakeOwnershipLatePreparation,
		Desc:         "Verifies that late-startup attestation can still be prepared for enrollment after taking ownership and still capable of removing owner dependency",
		Contacts:     []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		SoftwareDeps: []string{"reboot", "tpm"},
		Attr:         []string{"group:hwsec_destructive_func"},
		ServiceDeps:  []string{"tast.cros.hwsec.AttestationDBusService"},
		Params: []testing.Param{{
			Name: "uss",
			Val: retakeOwnershipLatePreparationWithAuthAPIParam{
				useUserSecretStash: true,
			},
		}, {
			Name: "vk",
			Val: retakeOwnershipLatePreparationWithAuthAPIParam{
				useUserSecretStash: false,
			},
		}}})
}

func RetakeOwnershipLatePreparation(ctx context.Context, s *testing.State) {
	userParam := s.Param().(retakeOwnershipLatePreparationWithAuthAPIParam)
	r := hwsecremote.NewCmdRunner(s.DUT())

	helper, err := hwsecremote.NewFullHelper(r, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	tpmManager := helper.TPMManagerClient()
	cryptohome := helper.CryptohomeClient()
	cryptohome.SetMountAPIParam(&hwsec.CryptohomeMountAPIParam{MountAPI: hwsec.AuthFactorMountAPI})
	if userParam.useUserSecretStash {
		// Enable the UserSecretStash experiment for the duration of the test by
		// creating a flag file that's checked by cryptohomed.
		cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
		if err != nil {
			s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
		}
		defer cleanupUSSExperiment(ctx)
	}

	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	dCtrl := helper.DaemonController()
	dCtrl.Stop(ctx, hwsec.AttestationDaemon)

	s.Log("Start taking ownership")
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure ownership: ", err)
	}
	s.Log("Ownership is taken")

	if passwd, err := tpmManager.GetOwnerPassword(ctx); err != nil {
		s.Fatal("Failed to get owner password: ", err)
	} else if len(passwd) != hwsec.OwnerPasswordLength {
		s.Fatalf("Unexpected owner password length: got %v; want %v", len(passwd), hwsec.OwnerPasswordLength)
	}

	s.Log("Start attestation service")
	dCtrl.Start(ctx, hwsec.AttestationDaemon)

	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Fatal("Failed to prepare for enrollment: ", err)
	}
	s.Log("Attestation is prepared for enrollment")

	s.Log("Clearing owner password")
	lastTime, err := r.Run(ctx, "stat", "-c", "%y", "/var/lib/tpm_manager/local_tpm_data")
	if err != nil {
		s.Log("Error calling stat; the polling operation will check the tpm password in every loop")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// This hacky logic watches the file modification of the persistent tpm status for both
		// monolithic and distributed models.
		// Ignores error here; if it's because file doesn't exist we assume the local data has changed.
		if _, err := tpmManager.ClearOwnerPassword(ctx); err != nil {
			return err
		}
		newTime, err := r.Run(ctx, "stat", "-c", "%y", "/var/lib/tpm_manager/local_tpm_data")
		if err == nil && bytes.Equal(lastTime, newTime) {
			return errors.New("no local data change")
		}
		lastTime = newTime
		// For now, restarting cryptohome is necessary because we still use cryptohome binary.
		if err := dCtrl.Restart(ctx, hwsec.CryptohomeDaemon); err != nil {
			return err
		}
		if passwd, err := tpmManager.GetOwnerPassword(ctx); err != nil {
			return err
		} else if len(passwd) != 0 {
			return errors.New("Still have password")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute}); err != nil {
		s.Fatal("Failed to wait for owner password to be cleared: ", err)
	}
}
