// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	libhwsec "chromiumos/tast/common/hwsec"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AttestationReuse,
		Desc: "Verifies certified system key",
		Attr: []string{"informational"},
		Contacts: []string{
			"cylai@chromium.org", // Nobody
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func AttestationReuse(ctx context.Context, s *testing.State) {
	s.Log("Start test with creating a helper and a utility")
	helper, err := libhwseclocal.NewHelperLocal()
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility, err := libhwsec.NewUtility(ctx, helper, libhwsec.CryptohomeBinaryType)
	// This pattern is so bad...smh. Need to find a better way to do the switch

	if err != nil {
		s.Error("Utilty creation error: ", err)
		return
	}
	if err := libhwsec.EnsureTpmIsReady(ctx, utility, defaultTakingOwnershipTimeout); err != nil {
		s.Error("Failed to ensure tpm readiness: ", err)
		return
	}
	s.Log("Tpm is ensured to be ready")
	if err := libhwsec.EnsureIsPreparedForEnrollment(ctx,
		utility, defaultPreparationForEnrolmentTimeout); err != nil {
		s.Error("Failed to prepare for enrollment: ", err)
		return
	}
	s.Log("Attestation is prepared for enrollment")
	// For now, just uses '0' as in "default CA"
	req, err := utility.CreateEnrollRequest(defaultACAType)
	s.Log(len(req))
	if err != nil {
		s.Error("Failed to create enroll request: ", err)
		return
	}
	//resp, err := sendPostRequestTo(req, "https://asbestos-qa.corp.google.com/enroll")
	resp, err := sendPostRequestTo(req, "https://chromeos-ca.gstatic.com/enroll")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	// For now, just uses '0' as in "default CA"
	err = utility.FinishEnroll(defaultACAType, resp)
	if err != nil {
		s.Error("Failed to finish enrollment: ", err)
	}
	isEnrolled, err := utility.IsEnrolled()
	if err != nil {
		s.Error("Failed to get enrollment status: ", err)
	}
	if !isEnrolled {
		s.Error("Inconsistent reported status: after enrollment, status shows 'not enrolled'")
	}
	s.Log("The DUT is enrolled")

	err = utility.FinishEnroll(defaultACAType, resp)
	if err != nil {
		s.Error("Failed to finish enrollment: ", err)
	}
	isEnrolled, err = utility.IsEnrolled()
	if err != nil {
		s.Error("Failed to get enrollment status: ", err)
	}
	if !isEnrolled {
		s.Error("Inconsistent reported status: after enrollment, status shows 'not enrolled'")
	}

	s.Log("Using the same repsonse to enroll again.")

	s.Log("Creating ceritificate request")

	// Empty username indicates the system-level key.
	username := ""

	req, err = utility.CreateCertRequest(
		defaultACAType,
		defaultCertProfile,
		username,
		defaultCertOrigin)
	if err != nil {
		s.Error("Failed to create certificate request: ", err)
	}
	s.Log("Created certificate request")

	s.Log("Sending sign request")
	resp, err = sendPostRequestTo(req, "https://chromeos-ca.gstatic.com/sign")
	//resp, err = sendPostRequestTo(req, "https://asbestos-qa.corp.google.com/sign")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	if len(resp) == 0 {
		s.Error("Unexpected empty cert")
		return
	}

	s.Log("Finishing certificate request")
	err = utility.FinishCertRequest(resp, username, defaultCertLabel)

	if err != nil {
		s.Error("Failed to finish cert request: ", err)
		return
	}
	s.Log("Finished certificate request")

	s.Log("Checking if FinishCert process can be replayed.")
	err = utility.FinishCertRequest(resp, username, defaultCertLabel)

	if err == nil {
		s.Error("FinishCert okay w/ the same response: ", err)
		return
	}
	s.Log("expected error for the reuse of the certificate response from server: ", err)
}
