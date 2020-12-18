// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AttestationEnrollOnly,
		Desc:     "Verifies attestation-related functionality",
		Attr:     []string{"group:mainline", "informational"},
		Contacts: []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		// Intentionally dependent on "chrome" so we can verify if the test is working in informational-chrome suite.
		SoftwareDeps: []string{"chrome", "tpm", "endorsement"},
	})
}

// AttestationEnrollOnly enrolls the device.
// Note that this item it to check if crbug/1070162 can be reproduced.
func AttestationEnrollOnly(ctx context.Context, s *testing.State) {
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
		s.Fatal("Failed to ensure tpm readiness: ", err)
	}
	s.Log("TPM is ensured to be ready")
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Fatal("Failed to prepare for enrollment: ", err)
	}

	at := hwsec.NewAttestationTest(utility, hwsec.DefaultPCA)
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
}
