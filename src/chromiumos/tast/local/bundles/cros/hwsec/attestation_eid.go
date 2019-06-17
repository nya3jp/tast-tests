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
		Desc:         "Verifies attestation-related functionality",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		SoftwareDeps: []string{"tpm"},
	})
}

func AttestationEID(ctx context.Context, s *testing.State) {
	r, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}

	enc, err := utility.GetEnrollmentID(ctx)
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
		s.Fatal("Expected size of EID in 32byte; hex-encoded EID: ", enc)
	}
}
