// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/stork"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HermesTestSmdsCI,
		Desc: "Perform SMDS eSIM operations on test eSIM",
		Contacts: []string{
			"srikanthkumar@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_sim_test_esim"},
		Timeout: 10 * time.Minute,
		Params: []testing.Param{{
			Name: "hermes_only",
			Val:  hermesconst.HermesOnly,
		}},
	})
}

func HermesTestSmdsCI(ctx context.Context, s *testing.State) {
	testMode, ok := s.Param().(string)
	if !ok {
		s.Fatal("Unable to read test mode")
	}
	if testMode == "" {
		s.Fatal("Add .hermes_only at the end of test name")
	}

	// Get a test euicc
	euicc, _, err := hermes.GetEUICC(ctx, true)
	if err != nil {
		s.Fatal("Unable to get Hermes euicc: ", err)
	}

	if err := euicc.Call(ctx, "UseTestCerts", true).Err; err != nil {
		s.Fatal("Failed to use test certs: ", err)
	}
	s.Log("Using test certs")

	if err := euicc.Call(ctx, hermesconst.EuiccMethodResetMemory, 1).Err; err != nil {
		s.Fatal("Failed to reset test euicc: ", err)
	}

	// Need to create EID based profile first then call RequestPendingProfiles and then do InstallPendingProfile.
	Eid, err := euicc.GetEid(ctx, false)
	if err != nil {
		s.Fatal("Failed to read euicc EID")
	}
	const numProfiles = 2
	profiles := make([]*hermes.Profile, numProfiles)
	for i := 0; i < numProfiles; i++ {
		activationCode, cleanupFunc, err := stork.FetchStorkProfileWithEid(ctx, Eid)
		if err != nil {
			s.Fatal("Failed to fetch Stork profile: ", err)
		}
		defer cleanupFunc(ctx)
		s.Log("Fetched Stork profile with activation code: ", activationCode)
	}

	pendingProfiles, err := euicc.PendingProfiles(ctx, false)
	if err != nil {
		s.Log("Failed to get pending profiles: ", err)
	}
	if len(pendingProfiles) < 1 {
		s.Log("No profiles found on euicc. Expected atleast one pending profile")
	}

	for i, profile := range pendingProfiles {
		s.Logf("Pending profile %s", profile.String())
		profiles[i] = installAndEnablePendingProfile(ctx, s, euicc, profile)
	}

	checkInstalledProfilesCnt(ctx, s, euicc, numProfiles)

	s.Log("Disabling profile ", profiles[numProfiles-1])
	if err := profiles[numProfiles-1].Call(ctx, hermesconst.ProfileMethodDisable).Err; err != nil {
		s.Fatal("Failed to disable profile: ", profiles[numProfiles-1])
	}
	if err := hermes.CheckProperty(ctx, profiles[numProfiles-1].DBusObject, hermesconst.ProfilePropertyState, int32(hermesconst.ProfileStateDisabled)); err != nil {
		s.Fatal("Failed to check profile state: ", err)
	}

	s.Log("Renaming profile ", profiles[0])
	if err := profiles[0].Call(ctx, hermesconst.ProfileMethodRename, "profile0").Err; err != nil {
		s.Fatal("Failed to rename profile: ", profiles[0])
	}
	hermes.CheckProperty(ctx, profiles[0].DBusObject, hermesconst.ProfilePropertyNickname, "profile0")

	s.Log("Uninstalling profile ", profiles[0])
	if err := euicc.Call(ctx, hermesconst.EuiccMethodUninstallProfile, profiles[0].DBusObject.ObjectPath()).Err; err != nil {
		s.Fatal("Failed to uninstall profile: ", profiles[0])
	}

	s.Log("Reset ", euicc)
	checkInstalledProfilesCnt(ctx, s, euicc, numProfiles-1)
	if err := euicc.Call(ctx, hermesconst.EuiccMethodResetMemory, 1).Err; err != nil {
		s.Fatal("Failed to reset test euicc: ", err)
	}

	checkInstalledProfilesCnt(ctx, s, euicc, 0)
	s.Log("Reset test euicc completed")
}

// installAndEnablePendingProfile installs and enables a profile.
func installAndEnablePendingProfile(ctx context.Context, s *testing.State, euicc *hermes.EUICC, pendingProfile hermes.Profile) (p *hermes.Profile) {
	s.Logf("Installing profile %s", p)
	response := euicc.Call(ctx, hermesconst.EuiccMethodInstallPendingProfile, pendingProfile, "")
	if response.Err != nil {
		s.Fatalf("Failed to install profile with %s: %s", pendingProfile, response.Err)
	}
	if len(response.Body) != 1 {
		s.Fatalf("InstallPendingProfile resulted in incorrect response len: %d", len(response.Body))
	}
	profilePath, ok := response.Body[0].(dbus.ObjectPath)
	if !ok {
		s.Fatal("Could not parse path for installed profile")
	}
	profile, err := hermes.NewProfile(ctx, profilePath)
	if err != nil {
		s.Fatal("Could not create dbus object")
	}
	s.Logf("Enabling profile %s", profile.String())
	if err := profile.Call(ctx, hermesconst.ProfileMethodEnable).Err; err != nil {
		s.Fatalf("Failed to enable %s: %s", profile.String(), err)
	}
	if err := hermes.CheckProperty(ctx, profile.DBusObject, hermesconst.ProfilePropertyState, int32(hermesconst.ProfileStateEnabled)); err != nil {
		s.Fatal("Failed to check profile state: ", err)
	}
	return profile
}

func checkInstalledProfilesCnt(ctx context.Context, s *testing.State, euicc *hermes.EUICC, expected int) {
	installedProfiles, err := euicc.InstalledProfiles(ctx, false)
	if err != nil {
		s.Fatal("Failed to get installed profiles: ", err)
	}
	if len(installedProfiles) != expected {
		s.Fatalf("Unexpected number of installed profiles, got: %d, want: %d", len(installedProfiles), expected)
	}
}
