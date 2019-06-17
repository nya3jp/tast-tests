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
	s.Log("Start test with creating a helper and a utility")
	helper, err := libhwseclocal.NewHelper()
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility, err := libhwsec.NewUtility(ctx, helper, libhwsec.CryptohomeBinaryType)

	s.Log("Switching to use of synchronous attestaton APIs")
	if err := utility.SetAttestationAsyncMode(false); err != nil {
		s.Fatal("Failed to switch to sync mode")
	}

	// This pattern is so bad...smh. Need to find a better way to do the switch
	if err != nil {
		s.Error("Utilty creation error: ", err)
		return
	}
	if err := libhwsec.EnsureTpmIsReady(ctx, utility, a9n.DefaultTakingOwnershipTimeout); err != nil {
		s.Error("Failed to ensure tpm readiness: ", err)
		return
	}
	s.Log("Tpm is ensured to be ready")
	if err := libhwsec.EnsureIsPreparedForEnrollment(ctx,
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
	libhwsec.RestartCryptohome(ctx, helper)
	if ownerPassword, err := utility.GetOwnerPassword(); err != nil {
		s.Fatal("Failed to get owner password: ", err)
	} else if ownerPassword != "" {
		s.Fatal("Still have owner password: ", ownerPassword)
	}
}
