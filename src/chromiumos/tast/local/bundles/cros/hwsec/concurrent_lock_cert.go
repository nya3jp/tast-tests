// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	libhwsec "chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/chrome"
	libhwseclocal "chromiumos/tast/local/hwsec"
	a9n "chromiumos/tast/local/hwsec/attestation"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConcurrentLockCert,
		Desc: "Stress tests lock and cert",
		Attr: []string{"informational"},
		Contacts: []string{
			"cylai@chromium.org", // Nobody
		},
		SoftwareDeps: []string{"chrome", "tpm"},
	})
}

// ConcurrentLockCert verified if concurrency of lock-unlock screen and cert process.
func ConcurrentLockCert(ctx context.Context, s *testing.State) {
	r, err := libhwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	helper, err := libhwseclocal.NewHelper()
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility, err := libhwsec.NewUtility(ctx, r, libhwsec.CryptohomeBinaryType)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	if err := helper.EnsureTPMIsReady(ctx, utility, a9n.DefaultTakingOwnershipTimeout); err != nil {
		s.Error("Failed to ensure tpm readiness: ", err)
		return
	}
	s.Log("TPM is ensured to be ready")
	if err := helper.EnsureIsPreparedForEnrollment(ctx,
		utility, a9n.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Error("Failed to prepare for enrollment: ", err)
		return
	}
	s.Log("Attestation is prepared for enrollment")

	enrollTask := libhwsec.NewStressTaskEnroll(utility)
	libhwsec.RegisterRunner(enrollTask, "enroll")

	passwd := "testpass"
	auth := chrome.Auth("test@crashwsec.bigr.name", passwd, "gaia-id")
	cr, err := chrome.New(ctx, auth)
	if err != nil {
		s.Fatal("Failed to log in by Chrome: ", err)
	}
	defer cr.Close(ctx)
	username := cr.User()
	s.Log("Chrome user: ", username)
	conn, err := cr.TestAPIConn(ctx)

	libhwsec.RegisterRunner(libhwseclocal.NewStressTaskLockScreen(ctx, conn, passwd), "lock")

	certTask := libhwsec.NewStressTaskCert(utility, username, "aaa")
	libhwsec.RegisterRunner(certTask, "cert")
	topJSON := `{
		"primary": {
			"names": ["enroll","cert"],
			"count": 5,
			"cof":false
		},
		"secondary": [
			{
				"names": ["lock"]
				"cof":true
			}
		]
	}`
	topTester, err := libhwsec.UnmarshalPSTaskModel(topJSON)
	if err != nil {
		s.Fatal("error creating top stress tester: ", err)
	}
	topTester.Run(nil)
}
