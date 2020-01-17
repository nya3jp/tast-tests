// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/hwsec/attestation"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AttestationExpiredCertReq,
		Desc:         "Verifies certified system key",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"cylai@chromium.org", "hwsec@google.com"},
		SoftwareDeps: []string{"chrome"},
	})
}

func AttestationExpiredCertReq(ctx context.Context, s *testing.State) {
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
	s.Log("Switching to use of synchronous attestaton APIs")
	if err := utility.SetAttestationAsyncMode(ctx, false); err != nil {
		s.Fatal("Failed to switch to sync mode")
	}
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Error("Failed to ensure tpm readiness: ", err)
		return
	}
	s.Log("TPM is ensured to be ready")
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Error("Failed to prepare for enrollment: ", err)
		return
	}
	s.Log("Attestation is prepared for enrollment")
	req, err := utility.CreateEnrollRequest(ctx, hwsec.DefaultPCA)
	s.Log(len(req))
	if err != nil {
		s.Error("Failed to create enroll request: ", err)
		return
	}
	//resp, err := attestation.SendPostRequestTo(ctx,req, "https://asbestos-qa.corp.google.com/enroll")
	resp, err := attestation.SendPostRequestTo(ctx, req, "https://chromeos-ca.gstatic.com/enroll")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	// For now, just uses '0' as in "default CA"
	err = utility.FinishEnroll(ctx, hwsec.DefaultPCA, resp)
	if err != nil {
		s.Error("Failed to finish enrollment: ", err)
	}
	isEnrolled, err := utility.IsEnrolled(ctx)
	if err != nil {
		s.Error("Failed to get enrollment status: ", err)
	}
	if !isEnrolled {
		s.Error("Inconsistent reported status: after enrollment, status shows 'not enrolled'")
	}
	s.Log("The DUT is enrolled")

	s.Log("Creating ceritificate request")

	// Empty username indicates the system-level key.
	username := ""

	req, err = utility.CreateCertRequest(
		ctx,
		hwsec.DefaultPCA,
		attestation.DefaultCertProfile,
		username,
		attestation.DefaultCertOrigin)
	if err != nil {
		s.Error("Failed to create certificate request: ", err)
	}
	s.Log("Created certificate request")

	s.Log("Enrolls again")

	req, err = utility.CreateEnrollRequest(ctx, hwsec.DefaultPCA)
	s.Log(len(req))
	if err != nil {
		s.Error("Failed to create enroll request: ", err)
		return
	}
	//resp, err = attestation.SendPostRequestTo(ctx,req, "https://asbestos-qa.corp.google.com/enroll")
	resp, err = attestation.SendPostRequestTo(ctx, req, "https://chromeos-ca.gstatic.com/enroll")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	// For now, just uses '0' as in "default CA"
	err = utility.FinishEnroll(ctx, hwsec.DefaultPCA, resp)
	if err != nil {
		s.Error("Failed to finish enrollment: ", err)
	}
	isEnrolled, err = utility.IsEnrolled(ctx)
	if err != nil {
		s.Error("Failed to get enrollment status: ", err)
	}
	if !isEnrolled {
		s.Error("Inconsistent reported status: after enrollment, status shows 'not enrolled'")
	}

	s.Log("The DUT is enrolled")

	s.Log("Sending expired cert request")
	resp, err = attestation.SendPostRequestTo(ctx, req, "https://chromeos-ca.gstatic.com/sign")
	//resp, err = attestation.SendPostRequestTo(ctx,req, "https://asbestos-qa.corp.google.com/sign")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	if len(resp) == 0 {
		s.Error("Unexpected empty cert")
		return
	}

	s.Log("Finishing certificate request")
	err = utility.FinishCertRequest(ctx, resp, username, attestation.DefaultCertLabel)

	if err == nil {
		s.Fatal("Expired cert managged to be finished")
	}
}
