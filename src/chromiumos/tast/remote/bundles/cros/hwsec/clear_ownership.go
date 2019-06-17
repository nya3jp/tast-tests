// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	libhwsec "chromiumos/tast/common/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ClearOwnership,
		Desc:         "Verifies that the TPM ownership can be cleared",
		Contacts:     []string{"cylai@google.com"},
		SoftwareDeps: []string{"reboot"},
		Attr:         []string{"informational"},
	})
}

func ClearOwnership(ctx context.Context, s *testing.State) {

	s.Log("Start test with creating a proxy")
	utility, err := libhwsec.NewUtility(ctx, s, libhwsec.CryptohomeBinaryType)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	s.Log("Start resetting TPM if needed")
	if err := libhwsec.EnsureTpmIsReset(ctx, s, utility); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	if result, err := utility.IsPreparedForEnrollment(); err != nil {
		s.Fatal("Cannot check if enrollment preparation is reset")
	} else if result {
		s.Fatal("Enrollment preparation is not reset after clearing ownership")
	}
	s.Log("Enrolling with TPM not ready")
	if _, err := utility.CreateEnrollRequest(0); err == nil {
		s.Fatal("Enrollment should not happen w/o getting prepared")
	}
}

