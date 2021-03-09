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
	r := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewFullHelper(ctx, r)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	attestation := helper.AttestationClient()

	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure tpm readiness: ", err)
	}
	s.Log("TPM is ensured to be ready")
	if err := helper.EnsureIsPreparedForEnrollment(ctx, hwsec.DefaultPreparationForEnrolmentTimeout); err != nil {
		s.Fatal("Failed to prepare for enrollment: ", err)
	}

	at := hwsec.NewAttestationTest(attestation, hwsec.DefaultPCA)

	if err := at.Enroll(ctx); err != nil {
		s.Fatal("Failed to enroll device: ", err)
	}
}
