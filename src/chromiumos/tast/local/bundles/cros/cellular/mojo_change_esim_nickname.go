// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/local/cellular/esim/mojo"
	"chromiumos/tast/local/stork"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MojoChangeESimNickname,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Installs a new eSIM profile on the device and then changes its nickname",
		Contacts: []string{
			"jstanko@google.com",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_test_esim"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithMojoTestEuicc",
		Timeout:      5 * time.Minute,
	})
}

const (
	// Test nickname to assign to the new profile.
	testName = "Test Nickname"
)

func MojoChangeESimNickname(ctx context.Context, s *testing.State) {
	eSimMojo := s.FixtValue().(*mojo.FixtData)

	activationCode, cleanupFunc, err := stork.FetchStorkProfile(ctx, "")
	if err != nil {
		s.Fatal("Failed to fetch Stork profile: ", err)
	}
	defer cleanupFunc(ctx)

	var euicc = eSimMojo.Euicc

	s.Log("Installing Stork profile with activation code: ", activationCode)
	installResult, profile, err := euicc.InstallProfileFromActivationCode(ctx, string(activationCode), "" /*confirmationCode*/)
	if err != nil {
		s.Fatal("Failed to install eSIM profile via Mojo: ", err)
	}
	if installResult != mojo.ProfileInstallSuccess {
		s.Fatal("eSIM install failed with error code: ", installResult)
	}

	s.Log("Installed Stork profile with ICCID: ", profile.Iccid)
	defer profile.UninstallProfile(ctx)

	s.Log("Changing profile nickname")
	_, err = profile.SetProfileNickname(ctx, mojo.NewString16(testName))
	if err != nil {
		s.Fatal("Failed to set eSIM profile nickname via Mojo: ", err)
	}

	profileProperties, err := profile.Properties(ctx)
	if err != nil {
		s.Fatal("Failed to get eSIM profile properties via Mojo: ", err)
	}

	if profileProperties.Nickname.String() != testName {
		s.Fatalf("Failed to confirm eSIM profile nickname, got: %v, want: %v",
			profileProperties.Nickname, testName)
	}
}
