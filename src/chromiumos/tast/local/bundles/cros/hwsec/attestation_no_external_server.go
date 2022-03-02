// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/testexec"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AttestationNoExternalServer,
		Desc:         "Verifies attestation-related functionality with the locally PCA and VA response",
		Attr:         []string{"group:mainline"},
		Contacts:     []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		SoftwareDeps: []string{"tpm"},
		Timeout:      4 * time.Minute,
	})
}

// isTPM2 checks if the DUT has a TPM2.0 implementation. In case of any error, |false| is returned.
func isTPM2(ctx context.Context) bool {
	out, err := testexec.CommandContext(ctx, "tpmc", "tpmversion").Output()
	// If tpmc is not available, assume it's TPM-less.
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "2.0"
}

// AttestationNoExternalServer runs through the attestation flow, including enrollment, cert, sign challenge.
// Also, it verifies the the key access functionality. All the external dependencies are replaced with the locally generated server responses.
func AttestationNoExternalServer(ctx context.Context, s *testing.State) {
	s.Log("Restarting ui job")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	r := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewFullHelper(ctx, r)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	attestation := helper.AttestationClient()
	cryptohome := helper.CryptohomeClient()
	mountInfo := hwsec.NewCryptohomeMountInfo(r, cryptohome)

	const username = "test@crashwsec.bigr.name"

	s.Log("Resetting vault in case the cryptohome status is contaminated")
	// Okay to call it even if the vault doesn't exist.
	if _, err := cryptohome.RemoveVault(ctx, username); err != nil {
		s.Fatal("Failed to cleanup: ", err)
	}

	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure tpm readiness: ", err)
	}

	ali := hwseclocal.NewAttestationLocalInfra(helper.DaemonController())
	if err := ali.Enable(ctx); err != nil {
		s.Fatal("Failed to enable local test infra feature: ", err)
	}
	defer func(ctx context.Context) {
		if err := ali.Disable(ctx); err != nil {
			s.Error("Failed to disable local test infra feature: ", err)
		}
	}(ctx)

	s.Log("TPM is ensured to be ready")
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Fatal("Failed to prepare for enrollment: ", err)
	}

	at := hwsec.NewAttestationTestWith(attestation, hwsec.DefaultPCA, hwseclocal.NewPCAAgentClient(), hwseclocal.NewLocalVA())

	ac, err := hwseclocal.NewAttestationDBus(ctx)
	if err != nil {
		s.Fatal("Failed to create attestation client: ", err)
	}

	enrollReply, err := ac.Enroll(ctx, &apb.EnrollRequest{Forced: proto.Bool(true)})
	if err != nil {
		s.Fatal("Failed to call Enroll D-Bus API: ", err)
	}
	if *enrollReply.Status != apb.AttestationStatus_STATUS_SUCCESS {
		s.Fatal("Faild to enroll: ", enrollReply.Status.String())
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
		keyType  apb.KeyType
	}{
		{
			name:     "system_cert",
			username: "",
		},
		{
			name:     "user_cert_rsa",
			username: username,
			keyType:  apb.KeyType_KEY_TYPE_RSA,
		},
		{
			name:     "user_cert_ecc",
			username: username,
			keyType:  apb.KeyType_KEY_TYPE_ECC,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if !isTPM2(ctx) && param.keyType == apb.KeyType_KEY_TYPE_ECC {
				s.Log("Skipping unsupported key type item: ", param.name)
				return
			}
			username := param.username
			certReply, err := ac.GetCertificate(ctx, &apb.GetCertificateRequest{Username: proto.String(username), KeyLabel: proto.String(hwsec.DefaultCertLabel), KeyType: &param.keyType})
			if err != nil {
				s.Fatal("Failed to call D-Bus API to get certificate: ", err)
			}
			if *certReply.Status != apb.AttestationStatus_STATUS_SUCCESS {
				s.Fatal("Faild to get certificate: ", certReply.Status.String())
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
					s.Fatalf("Faild to get certificate for label %q: %v", label, certReply.Status.String())
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
				if _, err := attestation.GetPublicKey(ctx, username, label); err == nil {
					s.Fatalf("key with label %q still found: %v", label, err)
				}
			}
			s.Log("Deletion of keys by prefix verified")
		})
	}
}
