// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	libhwsec "chromiumos/tast/common/hwsec"
	libhwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RetakeOwnershipFlushPassword,
		Desc: "Verifies that taking ownership produce a new owner password",
		Contacts: []string{
			"cylai@chromium.org", // Nobody
		},
		SoftwareDeps: []string{"reboot", "tpm"},
		Attr:         []string{"informational"},
	})
}

func RetakeOwnershipFlushPassword(ctx context.Context, s *testing.State) {
	s.Log("Start test with creating a helper and a utility")
	helper, err := libhwsecremote.NewHelperRemote(s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility, err := libhwsec.NewUtility(ctx, &helper.Helper, libhwsec.CryptohomeBinaryType)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMIsReset(ctx, utility); err != nil {
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

	s.Log("Start taking ownership")
	if err := libhwsec.EnsureTPMIsReady(ctx, utility, libhwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure ownership: ", err)
	}
	s.Log("Onwership is taken")

	passwd, err := utility.GetOwnerPassword()
	if err != nil {
		s.Fatal("Failed to get owner password")
	} else if len(passwd) != 20 {
		s.Fatal("Ill-formed owner password")
	}
	s.Log("Start resetting TPM again")
	if err := helper.EnsureTPMIsReset(ctx, utility); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	passwd2, err := utility.GetOwnerPassword()
	if err != nil {
		s.Fatal("Failed to get owner password")
	} else if len(passwd2) != 0 {
		s.Fatal("Non-empty owner password after reset")
	}

	s.Log("Start taking ownership again")
	if err := libhwsec.EnsureTPMIsReady(ctx, utility, libhwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure ownership: ", err)
	}
	s.Log("Onwership is taken")

	passwd2, err = utility.GetOwnerPassword()
	if err != nil {
		s.Fatal("Failed to get owner password")
	} else if len(passwd) != 20 {
		s.Fatal("Ill-formed owner password")
	}
	if passwd == passwd2 {
		s.Fatal("Owner password doesn't get re-created")
	}

}
