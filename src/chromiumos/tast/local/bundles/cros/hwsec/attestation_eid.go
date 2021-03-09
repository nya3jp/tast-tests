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
		Func:         AttestationEID,
		Desc:         "Verifies that enrollment ID is available",
		Attr:         []string{"group:mainline"},
		Contacts:     []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		SoftwareDeps: []string{"tpm"},
	})
}

func AttestationEID(ctx context.Context, s *testing.State) {
	r := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewFullHelper(ctx, r)
	if err != nil {
		s.Fatal("Local hwsec helper creation error: ", err)
	}
	attestation := helper.AttestationClient()

	// Enrollment ID depends on endorsement key, which can only be read when TPM is ready on TPMv1.2.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure TPM readiness: ", err)
	}

	enc, err := attestation.GetEnrollmentID(ctx)
	if err != nil {
		s.Fatal("Failed to get enrollment id: ", err)
	}
	dec, err := hwsec.HexDecode([]byte(enc))
	if err != nil {
		s.Fatal("Failed to decode eid: ", err)
	}
	// SHA-256 digest size in bytes.
	if len(dec) != 32 {
		// Prints hex-encoded data because the decoded result might not be readable.
		s.Fatal("Expected size of EID in 32 bytes; hex-encoded EID: ", enc)
	}
}
