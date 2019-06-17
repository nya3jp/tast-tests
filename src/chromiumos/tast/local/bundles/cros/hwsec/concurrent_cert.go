// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/chrome"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConcurrentCert,
		Desc: "Stress tests lock and cert",
		Attr: []string{"informational"},
		Contacts: []string{
			"cylai@chromium.org", // Nobody
		},
		SoftwareDeps: []string{"chrome", "tpm"},
	})
}

// ConcurrentCert verified concurrency of multiple certs.
func ConcurrentCert(ctx context.Context, s *testing.State) {
	r, err := libhwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	helper, err := libhwseclocal.NewHelper(utility)
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

	enrollTask := hwsec.NewStressTaskEnroll(utility)
	if err := enrollTask.RunTask(ctx); err != nil {
		s.Fatal("Failed to enroll: ", err)
	}

	passwd := "testpass"
	auth := chrome.Auth("test@crashwsec.bigr.name", passwd, "gaia-id")
	cr, err := chrome.New(ctx, auth)
	if err != nil {
		s.Fatal("Failed to log in by Chrome: ", err)
	}
	defer cr.Close(ctx)
	username := cr.User()
	s.Log("Chrome user: ", username)

	certTask := hwsec.NewStressTaskCert(utility, "", "sys-cert")
	hwsec.RegisterRunner(certTask, "syscert")

	for i := 0; i < 5; i++ {
		certTask := hwsec.NewStressTaskCert(utility, username, fmt.Sprintf("cert%v", i))
		hwsec.RegisterRunner(certTask, fmt.Sprintf("cert%v", i))
	}

	topJSON := `{
		"primary": {
			"names": ["syscert"],
			"count": 7,
			"cof":false
		},
		"secondary": [
			{
				"names": ["cert0"],
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
