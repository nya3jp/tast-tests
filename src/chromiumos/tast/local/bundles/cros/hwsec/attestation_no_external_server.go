// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AttestationNoExternalServer,
		Desc:         "Verifies attestation-related functionality with the locally PCA and VA response",
		Contacts:     []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		SoftwareDeps: []string{"tpm"},
		Timeout:      4 * time.Minute,
	})
}

// AttestationNoExternalServer runs through the attestation flow, including enrollment, cert, sign challenge.
// Also, it verifies the the key access functionality. All the external dependencies are replaced with the locally generated server responses.
// TODO(b/154672162): Replace the communication with real servers with fake responses generated locally on DUT.
func AttestationNoExternalServer(ctx context.Context, s *testing.State) {
	r, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	helper, err := hwseclocal.NewHelper(utility)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	ali := hwseclocal.NewAttestationLocalInfra(hwsec.NewDaemonController(r))
	if err := ali.Enable(ctx); err != nil {
		s.Fatal("Failed to enable local test infra feature: ", err)
	}
	defer func() {
		if err := ali.Disable(ctx); err != nil {
			s.Error("Failed to disable local test infra feature: ", err)
		}
	}()

	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure tpm readiness: ", err)
	}
	s.Log("TPM is ensured to be ready")
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Fatal("Failed to prepare for enrollment: ", err)
	}

	at := hwsec.NewAttestaionTestWith(utility, hwsec.DefaultPCA, hwseclocal.NewPCAAgentClient(), hwseclocal.NewLocalVA())
	for _, param := range []struct {
		name  string
		async bool
	}{
		{
			name:  "async_enroll",
			async: true,
		}, {
			name:  "sync_enroll",
			async: false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := utility.SetAttestationAsyncMode(ctx, param.async); err != nil {
				s.Fatal("Failed to switch to sync mode: ", err)
			}
			if err := at.Enroll(ctx); err != nil {
				s.Fatal("Failed to enroll device: ", err)
			}
		})
	}

	const username = "test@crashwsec.bigr.name"

	resetVault := func() {
		if _, err := utility.Unmount(ctx, username); err != nil {
			s.Fatal("Failed to remove user vault: ", err)
		}
		if _, err := utility.RemoveVault(ctx, username); err != nil {
			s.Fatal("Failed to remove user vault: ", err)
		}
	}

	s.Log("Resetting vault in case the cryptohome status is contaminated")
	// Okay to call it even if the vault doesn't exist.
	resetVault()

	if err := utility.MountVault(ctx, username, "testpass", "dummy_label", true /* create */, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user vault: ", err)
	}

	defer func() {
		s.Log("Resetting vault after use")
		resetVault()
	}()

	for _, param := range []struct {
		name     string
		async    bool
		username string
	}{
		{
			name:     "async_system_cert",
			async:    true,
			username: "",
		},
		{
			name:     "async_user_cert",
			async:    true,
			username: username,
		}, {
			name:     "sync_user_cert",
			async:    false,
			username: username,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := utility.SetAttestationAsyncMode(ctx, param.async); err != nil {
				s.Fatal("Failed to switch to sync mode: ", err)
			}
			username := param.username
			if err := at.GetCertificate(ctx, username, hwsec.DefaultCertLabel); err != nil {
				s.Fatal("Failed to enroll device: ", err)
			}

			if username != "" {
				if err := at.SignEnterpriseChallenge(ctx, username, hwsec.DefaultCertLabel); err != nil {
					s.Fatal("Failed to sign enterprise challenge: ", err)
				}
			}

			if err := at.SignSimpleChallenge(ctx, username, hwsec.DefaultCertLabel); err != nil {
				s.Fatal("Failed to sign simple challenge: ", err)
			}

			s.Log("Start key payload closed-loop testing")
			s.Log("Setting key payload")
			expectedPayload := hwsec.DefaultKeyPayload
			_, err = utility.SetKeyPayload(ctx, username, hwsec.DefaultCertLabel, expectedPayload)
			if err != nil {
				s.Fatal("Failed to set key payload: ", err)
			}
			s.Log("Getting key payload")
			resultPayload, err := utility.GetKeyPayload(ctx, username, hwsec.DefaultCertLabel)
			if err != nil {
				s.Fatal("Failed to get key payload: ", err)
			}
			if resultPayload != expectedPayload {
				s.Fatalf("Inconsistent paylaod -- result: %s / expected: %s", resultPayload, expectedPayload)
			}
			s.Log("Start key payload closed-loop done")

			s.Log("Start verifying key registration")
			isSuccessful, err := utility.RegisterKeyWithChapsToken(ctx, username, hwsec.DefaultCertLabel)
			if err != nil {
				s.Fatal("Failed to register key with chaps token due to error: ", err)
			}
			if !isSuccessful {
				s.Fatal("Failed to register key with chaps token")
			}
			// Now the key has been registered and remove from the key store
			_, err = utility.GetPublicKey(ctx, username, hwsec.DefaultCertLabel)
			if err == nil {
				s.Fatal("unsidered successful operation -- key should be removed after registration")
			}
			// Well, actually we need more on system key so the key registration is validated.
			s.Log("Key registration verified")

			s.Log("Verifying deletion of keys by prefix")
			for _, label := range []string{"label1", "label2", "label3"} {
				if err := at.GetCertificate(ctx, username, label); err != nil {
					s.Fatalf("Failed to create certificate request for label %q: %v", label, err)
				}
				_, err = utility.GetPublicKey(ctx, username, label)
				if err != nil {
					s.Fatalf("Failed to get public key for label %q: %v", label, err)
				}
			}
			s.Log("Deleting keys just created")
			if err := utility.DeleteKeys(ctx, username, "label"); err != nil {
				s.Fatal("Failed to remove the key group: ", err)
			}
			for _, label := range []string{"label1", "label2", "label3"} {
				if _, err := utility.GetPublicKey(ctx, username, label); err == nil {
					s.Fatalf("key with label %q still found: %v", label, err)
				}
			}
			s.Log("Deletion of keys by prefix verified")
		})
	}
}
