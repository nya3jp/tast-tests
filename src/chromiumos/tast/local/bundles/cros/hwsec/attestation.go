// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Attestation,
		Desc: "Verifies attestation-related functionality",
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "auth_session_api",
			Val:  &hwsec.CryptohomeMountAPIParam{MountAPI: hwsec.AuthSessionMountAPI},
		}, {
			Name: "auth_factor_api",
			Val:  &hwsec.CryptohomeMountAPIParam{MountAPI: hwsec.AuthFactorMountAPI},
		}},
		Contacts:     []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		SoftwareDeps: []string{"tpm", "endorsement"},
		Timeout:      4 * time.Minute,
	})
}

// Attestation runs through the attestation flow, including enrollment, cert, sign challenge.
// Also, it verifies the the key access functionality.
func Attestation(ctx context.Context, s *testing.State) {
	r := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewFullHelper(ctx, r)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	attestation := helper.AttestationClient()
	cryptohome := helper.CryptohomeClient()
	cryptohome.SetMountAPIParam(s.Param().(*hwsec.CryptohomeMountAPIParam))
	mountInfo := hwsec.NewCryptohomeMountInfo(r, cryptohome)
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure tpm readiness: ", err)
	}
	s.Log("TPM is ensured to be ready")
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Fatal("Failed to prepare for enrollment: ", err)
	}

	at := hwsec.NewAttestationTest(attestation, hwsec.DefaultPCA)

	ac, err := hwseclocal.NewAttestationDBus(ctx)
	if err != nil {
		s.Fatal("Failed to create attestation client: ", err)
	}

	enrollReply, err := ac.Enroll(ctx, &apb.EnrollRequest{Forced: proto.Bool(true)})
	if err != nil {
		s.Fatal("Failed to call Enroll D-Bus API: ", err)
	}
	if *enrollReply.Status != apb.AttestationStatus_STATUS_SUCCESS {
		s.Fatal("Failed to enroll: ", enrollReply.Status.String())
	}

	const username = "test@crashwsec.bigr.name"

	s.Log("Resetting vault in case the cryptohome status is contaminated")
	// Okay to call it even if the vault doesn't exist.
	if err := mountInfo.CleanUpMount(ctx, username); err != nil {
		s.Fatal("Failed to cleanup: ", err)
	}

	if err := cryptohome.MountVault(ctx, "fake_label", hwsec.NewPassAuthConfig(username, "testpass"), true /* create */, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user vault: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Resetting vault after use")
		if err := mountInfo.CleanUpMount(ctx, username); err != nil {
			s.Error("Failed to cleanup: ", err)
		}
	}(ctx)

	for _, param := range []struct {
		name     string
		username string
	}{
		{
			name:     "system_cert",
			username: "",
		},
		{
			name:     "user_cert",
			username: username,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			username := param.username
			certReply, err := ac.GetCertificate(ctx, &apb.GetCertificateRequest{Username: proto.String(username), KeyLabel: proto.String(hwsec.DefaultCertLabel)})
			if err != nil {
				s.Fatal("Failed to call D-Bus API to get certificate: ", err)
			}
			if *certReply.Status != apb.AttestationStatus_STATUS_SUCCESS {
				s.Fatal("Failed to get certificate: ", certReply.Status.String())
			}

			// TODO(b/165426637): Enable it after we inject the fake device policy with customer ID.
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
			_, err = attestation.SetKeyPayload(ctx, username, hwsec.DefaultCertLabel, expectedPayload)
			if err != nil {
				s.Fatal("Failed to set key payload: ", err)
			}
			s.Log("Getting key payload")
			resultPayload, err := attestation.GetKeyPayload(ctx, username, hwsec.DefaultCertLabel)
			if err != nil {
				s.Fatal("Failed to get key payload: ", err)
			}
			if resultPayload != expectedPayload {
				s.Fatalf("Inconsistent paylaod -- result: %s / expected: %s", resultPayload, expectedPayload)
			}
			s.Log("Start key payload closed-loop done")

			s.Log("Start verifying key registration")
			isSuccessful, err := attestation.RegisterKeyWithChapsToken(ctx, username, hwsec.DefaultCertLabel)
			if err != nil {
				s.Fatal("Failed to register key with chaps token due to error: ", err)
			}
			if !isSuccessful {
				s.Fatal("Failed to register key with chaps token")
			}
			// Now the key has been registered and remove from the key store
			_, err = attestation.GetPublicKey(ctx, username, hwsec.DefaultCertLabel)
			if err == nil {
				s.Fatal("unsidered successful operation -- key should be removed after registration")
			}
			// Well, actually we need more on system key so the key registration is validated.
			s.Log("Key registration verified")

			s.Log("Verifying deletion of keys by prefix")
			for _, label := range []string{"label1", "label2", "label3"} {
				certReply, err := ac.GetCertificate(ctx, &apb.GetCertificateRequest{Username: proto.String(username), KeyLabel: proto.String(label)})
				if err != nil {
					s.Fatalf("Failed to create certificate request for label %q: %v", label, err)
				}
				if *certReply.Status != apb.AttestationStatus_STATUS_SUCCESS {
					s.Fatalf("Failed to get certificate for label %q: %v", label, certReply.Status.String())
				}
				_, err = attestation.GetPublicKey(ctx, username, label)
				if err != nil {
					s.Fatalf("Failed to get public key for label %q: %v", label, err)
				}
			}
			s.Log("Deleting keys just created")
			if err := attestation.DeleteKeys(ctx, username, "label"); err != nil {
				s.Fatal("Failed to remove the key group: ", err)
			}
			for _, label := range []string{"label1", "label2", "label3"} {
				_, err := attestation.GetPublicKey(ctx, username, label)
				var ae *hwsec.AttestationError
				if err == nil {
					s.Fatalf("Key with label %q still found", label)
				}
				if !errors.As(err, &ae) {
					s.Fatalf("Failed to get public key with label %q: %v", label, err)
				}
				if ae.AttestationStatus != apb.AttestationStatus_STATUS_INVALID_PARAMETER {
					s.Fatalf("Unexpected error status: got %s; want STATUS_INVALID_PARAMETER", ae.AttestationStatus.String())
				}
			}
			s.Log("Deletion of keys by prefix verified")
		})
	}
}
