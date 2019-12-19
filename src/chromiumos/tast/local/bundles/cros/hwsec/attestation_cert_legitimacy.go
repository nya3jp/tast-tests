// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/chrome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AttestationCertLegitimacy,
		Desc:         "Verifies login is the pre-condition of cert and test expired cert can't be finished",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		SoftwareDeps: []string{"chrome"},
	})
}

func AttestationCertLegitimacy(ctx context.Context, s *testing.State) {
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
	// For now, just uses '0' as in "default CA"
	req, err := utility.CreateEnrollRequest(ctx, hwsec.DefaultPCA)
	s.Log(len(req))
	if err != nil {
		s.Fatal("Failed to create enroll request: ", err)
	}
	//resp, err := hwsec.SendPostRequestTo(ctx,req, "https://asbestos-qa.corp.google.com/enroll")
	resp, err := hwsec.SendPostRequestTo(ctx, req, "https://chromeos-ca.gstatic.com/enroll")
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

	username := "test@crashwsec.bigr.name"

	req, err = utility.CreateCertRequest(
		ctx,
		hwsec.DefaultPCA,
		hwsec.DefaultCertProfile,
		username,
		hwsec.DefaultCertOrigin)
	if err != nil {
		s.Error("Failed to create certificate request: ", err)
	}
	s.Log("Created certificate request")

	s.Log("Sending sign request")
	resp, err = hwsec.SendPostRequestTo(ctx, req, "https://chromeos-ca.gstatic.com/sign")
	//resp, err = hwsec.SendPostRequestTo(ctx,req, "https://asbestos-qa.corp.google.com/sign")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	if len(resp) == 0 {
		s.Fatal("Unexpected empty cert")
	}

	s.Log("Finishing certificate request w/o login")
	err = utility.FinishCertRequest(ctx, resp, username, hwsec.DefaultCertLabel)

	if err == nil {
		s.Error("Expecting failure during finishing user cert: ", err)
		return
	}
	// auth := chrome.Auth("cros@crosdmsregtest.com", "testpass", "gaia-id")
	auth := chrome.Auth("test@crashwsec.bigr.name", "testpass", "gaia-id")
	cr, err := chrome.New(ctx, auth)
	if err != nil {
		s.Fatal("Failed to log in by Chrome: ", err)
	}
	defer cr.Close(ctx)
	if username != cr.User() {
		s.Fatal("Inconsistent username for cert and login")
	}
	s.Log("Chrome user: ", username)

	s.Log("Finishing expired certificate request")
	err = utility.FinishCertRequest(ctx, resp, username, hwsec.DefaultCertLabel)
	if err == nil {
		s.Error("Expecting failure of finishing expired cert: ", err)
		return
	}

	s.Log("Finishing certificate request after login")
	s.Log("Creating ceritificate request")

	req, err = utility.CreateCertRequest(
		ctx,
		hwsec.DefaultPCA,
		hwsec.DefaultCertProfile,
		username,
		hwsec.DefaultCertOrigin)
	if err != nil {
		s.Error("Failed to create certificate request: ", err)
	}
	s.Log("Created certificate request")

	s.Log("Sending sign request")
	resp, err = hwsec.SendPostRequestTo(ctx, req, "https://chromeos-ca.gstatic.com/sign")
	//resp, err = hwsec.SendPostRequestTo(ctx,req, "https://asbestos-qa.corp.google.com/sign")
	if err != nil {
		s.Error("Failed to send request to CA: ", err)
	}
	if len(resp) == 0 {
		s.Fatal("Unexpected empty cert")
	}

	s.Log("Finishing certificate request")
	err = utility.FinishCertRequest(ctx, resp, username, hwsec.DefaultCertLabel)

	if err != nil {
		s.Error("Failed to finish user cert: ", err)
		return
	}

	s.Log("Finished certificate request")
}
