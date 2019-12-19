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
		Func:         ConcurrentLockCert,
		Desc:         "Stress tests lock and cert",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		SoftwareDeps: []string{"chrome", "tpm"},
	})
}

// ConcurrentLockCert verified if concurrency of lock-unlock screen and cert process.
func ConcurrentLockCert(ctx context.Context, s *testing.State) {
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

	enrollTask := hwsec.NewStressTaskEnroll(utility)
	hwsec.RegisterRunner(enrollTask, "enroll")

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

	hwsec.RegisterRunner(hwseclocal.NewStressTaskLockScreen(ctx, conn, passwd), "lock")

	certTask := hwsec.NewStressTaskCert(utility, username, "aaa")
	hwsec.RegisterRunner(certTask, "cert")
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
	topTester, err := hwsec.UnmarshalPSTaskModel(topJSON)
	if err != nil {
		s.Fatal("error creating top stress tester: ", err)
	}
	topTester.Run(ctx, nil)
}
