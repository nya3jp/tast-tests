// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	hwsecremote "chromiumos/tast/remote/hwsec"
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
	r, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	helper, err := hwsecremote.NewHelper(utility, r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	if result, err := utility.IsPreparedForEnrollment(ctx); err != nil {
		s.Fatal("Cannot check if enrollment preparation is reset")
	} else if result {
		s.Fatal("Enrollment preparation is not reset after clearing ownership")
	}
	s.Log("Enrolling with TPM not ready")
	if _, err := utility.CreateEnrollRequest(ctx, hwsec.DefaultPCA); err == nil {
		s.Fatal("Enrollment should not happen w/o getting prepared")
	}

	s.Log("Start taking ownership")
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure ownership: ", err)
	}
	s.Log("Ownership is taken")

	passwd, err := utility.GetOwnerPassword(ctx)
	if err != nil {
		s.Fatal("Failed to get owner password")
	} else if len(passwd) != 20 {
		s.Fatal("Ill-formed owner password")
	}
	s.Log("Start resetting TPM again")
	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	passwd2, err := utility.GetOwnerPassword(ctx)
	if err != nil {
		s.Fatal("Failed to get owner password")
	} else if len(passwd2) != 0 {
		s.Fatal("Non-empty owner password after reset")
	}

	s.Log("Start taking ownership again")
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure ownership: ", err)
	}
	s.Log("Ownership is taken")

	passwd2, err = utility.GetOwnerPassword(ctx)
	if err != nil {
		s.Fatal("Failed to get owner password")
	} else if len(passwd) != 20 {
		s.Fatal("Ill-formed owner password")
	}
	if passwd == passwd2 {
		s.Fatal("Owner password doesn't get re-created")
	}

}
