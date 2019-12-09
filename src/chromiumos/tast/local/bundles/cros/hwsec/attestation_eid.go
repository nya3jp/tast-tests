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
		Func: AttestationEID,
		Desc: "Verifies attestation-related functionality",
		Attr: []string{"informational"},
		Contacts: []string{
			"cylai@chromium.org", // Nobody
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func AttestationEID(ctx context.Context, s *testing.State) {
	r, err := libhwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := libhwsec.NewUtility(ctx, r, libhwsec.CryptohomeBinaryType)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	s.Log("Getting enrollment id")
	if enc, err := utility.GetEnrollmentID(); err != nil {
		s.Fatal("Failed to get enrollment id: ", err)
	} else if dec, err := a9n.HexDecode([]byte(enc)); err != nil {
		s.Log(enc, " ", len(enc))

		s.Fatal("Failed to decode eid: ", err)
	} else if len(dec) != 32 { // SHA-256 digest size in bytes
		s.Fatal("Expected size of EID: ", len(dec))
	}
	s.Log("Getting enrollment id...Done")
}
