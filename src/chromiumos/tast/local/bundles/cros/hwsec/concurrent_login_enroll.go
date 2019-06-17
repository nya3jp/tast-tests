// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	libhwsec "chromiumos/tast/common/hwsec"
	libhwseclocal "chromiumos/tast/local/hwsec"
	a9n "chromiumos/tast/local/hwsec/attestation"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConcurrentLoginEnroll,
		Desc: "Stress tests login and enroll",
		Attr: []string{"informational"},
		Contacts: []string{
			"cylai@chromium.org", // Nobody
		},
		SoftwareDeps: []string{"chrome", "tpm"},
	})
}

// ConcurrentLoginEnroll verifies the concurrency of login and enrollment.
func ConcurrentLoginEnroll(ctx context.Context, s *testing.State) {
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
	enrollStressTester := libhwsec.NewSimpleStressTester(enrollTask, "enroll", 5, true)
	enrollStressTester.Run(nil)
}
