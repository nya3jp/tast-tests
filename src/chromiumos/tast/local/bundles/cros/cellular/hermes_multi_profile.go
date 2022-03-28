// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HermesMultiProfile,
		Desc: "Iterates over profiles in an eUICC and enables them. At least 1 profile must be preinstalled",
		Contacts: []string{
			"pholla@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_unstable", "cellular_sim_prod_esim"},
		Timeout: 10 * time.Minute,
	})
}

func HermesMultiProfile(ctx context.Context, s *testing.State) {

	euicc, _, err := hermes.GetEUICC(ctx, false)
	if err != nil {
		s.Fatal("Unable to get Hermes euicc: ", err)
	}

	s.Log("Looking for enabled profile before test begins")
	p, err := euicc.EnabledProfile(ctx)
	if err != nil {
		s.Fatal("Could not read profile status: ", err)
	}

	// Disable all profiles before starting the test
	if p != nil {
		s.Logf("Disabling profile %s", p.String())
		if err := p.Call(ctx, hermesconst.ProfileMethodDisable).Err; err != nil {
			s.Fatalf("Failed to disable %s: %s", p.String(), err)
		}
	}

	profiles, err := euicc.InstalledProfiles(ctx, true)
	if err != nil {
		s.Fatal("Failed to get installed profiles: ", err)
	}
	if len(profiles) < 1 {
		s.Fatal("No profiles found on euicc. Expected atleast one installed profile")
	}

	for _, profile := range profiles {
		s.Logf("Enabling profile %s", profile.String())
		if err := profile.Call(ctx, hermesconst.ProfileMethodEnable).Err; err != nil {
			s.Fatalf("Failed to enable %s: %s", profile.String(), err)
		}
		if err := hermes.CheckProperty(ctx, profile.DBusObject, hermesconst.ProfilePropertyState, int32(hermesconst.ProfileStateEnabled)); err != nil {
			s.Fatal("Failed to check profile state: ", err)
		}
		s.Logf("Disabling profile %s", profile.String())
		if err := profile.Call(ctx, hermesconst.ProfileMethodDisable).Err; err != nil {
			s.Fatalf("Failed to disable %s: %s", profile.String(), err)
		}
		if err := hermes.CheckProperty(ctx, profile.DBusObject, hermesconst.ProfilePropertyState, hermesconst.ProfileStateDisabled); err != nil {
			s.Fatal("Failed to check profile state: ", err)
		}
	}

	s.Log("Enabling profiles back to back without disabling them")
	for _, profile := range profiles {
		v, err := profile.IsTestProfile(ctx)
		if err != nil {
			s.Fatalf("Failed to check class of %s: %s", profile.String(), err)
		}
		if v {
			// certain eUICC's have trouble implicitly disabling test profiles
			// thus we skip them. These eUICC's will need a FW update.
			// Blocked on b/169946663. Workaround the issue since test profiles
			// are not visible to the user.
			continue
		}
		s.Logf("Enabling profile %s", profile.String())
		if err := profile.Call(ctx, hermesconst.ProfileMethodEnable).Err; err != nil {
			s.Fatalf("Failed to enable %s: %s", profile.String(), err)
		}
		if err := hermes.CheckProperty(ctx, profile.DBusObject, hermesconst.ProfilePropertyState, int32(hermesconst.ProfileStateEnabled)); err != nil {
			s.Fatal("Failed to check profile state: ", err)
		}
	}
}
