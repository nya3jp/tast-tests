// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	libhwsec "chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/chrome"
	libhwseclocal "chromiumos/tast/local/hwsec"
	a9n "chromiumos/tast/local/hwsec/attestation"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PasswordClearedAfterLogin,
		Desc: "Verifies synchronous attestation APIs",
		Attr: []string{"informational"},
		Contacts: []string{
			"cylai@chromium.org", // Nobody
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func PasswordClearedAfterLogin(ctx context.Context, s *testing.State) {
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
	auth := chrome.Auth("test@crashwsec.bigr.name", "testpass", "gaia-id")
	cr, err := chrome.New(ctx, auth)
	if err != nil {
		s.Fatal("Failed to log in by Chrome: ", err)
	}
	defer cr.Close(ctx)
	username := cr.User()
	s.Log("Chrome user: ", username)

	s.Log("Checking if onwer password is gone")
	//Needed when using legacy cryptohome interface.
	libhwsec.RestartCryptohome(ctx, r)
	if ownerPassword, err := utility.GetOwnerPassword(); err != nil {
		s.Fatal("Failed to get owner password: ", err)
	} else if ownerPassword != "" {
		s.Fatal("Still have owner password: ", ownerPassword)
	}
}
