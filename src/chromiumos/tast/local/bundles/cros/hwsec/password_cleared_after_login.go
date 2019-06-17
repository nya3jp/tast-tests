// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	libhwsec "chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/chrome"
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
	s.Log("Start test with creating a proxy")
	utility, err := libhwsec.NewUtility(ctx, libhwsec.CryptohomeBinaryType)

	s.Log("Switching to use of synchronous attestaton APIs")
	if err := utility.SetAttestationAsyncMode(false); err != nil {
		s.Fatal("Failed to switch to sync mode")
	}

	// This pattern is so bad...smh. Need to find a better way to do the switch
	if err != nil {
		s.Error("Utilty creation error: ", err)
		return
	}
	if err := libhwsec.EnsureTpmIsReady(ctx, utility, defaultTakingOwnershipTimeout); err != nil {
		s.Error("Failed to ensure tpm readiness: ", err)
		return
	}
	s.Log("Tpm is ensured to be ready")
	if err := libhwsec.EnsureIsPreparedForEnrollment(ctx,
		utility, defaultPreparationForEnrolmentTimeout); err != nil {
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

	s.Log("Checking if onwer password is gone...")
	//Needed when using legacy cryptohome interface.
	libhwsec.RestartCryptohome(ctx)
	if ownerPassword, err := utility.GetOwnerPassword(); err != nil {
		s.Fatal("Failed to get owner password: ", err)
	} else if ownerPassword != "" {
		s.Fatal("Still have owner password: ", ownerPassword)
	}
}
