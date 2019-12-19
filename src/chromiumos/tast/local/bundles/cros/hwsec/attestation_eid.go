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
		SoftwareDeps: []string{"chrome"},
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
	s.Log("Getting enrollment id")
	if enc, err := utility.GetEnrollmentID(ctx); err != nil {
		s.Fatal("Failed to get enrollment id: ", err)
	} else if dec, err := hwsec.HexDecode([]byte(enc)); err != nil {
		s.Log(enc, " ", len(enc))

		s.Fatal("Failed to decode eid: ", err)
	} else if len(dec) != 32 { // SHA-256 digest size in bytes
		s.Fatal("Expected size of EID: ", len(dec))
	}
	s.Log("Getting enrollment id...Done")
}
