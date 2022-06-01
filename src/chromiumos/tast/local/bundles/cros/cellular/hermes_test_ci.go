// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/stork"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HermesTestCI,
		Desc: "Perform eSIM operations on test eSIM",
		Contacts: []string{
			"pholla@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_sim_test_esim"},
		Timeout: 10 * time.Minute,
		Params: []testing.Param{{
			Name: "hermes_only",
			Val:  hermesconst.HermesOnly,
		}, {
			Name: "hermes_and_mm",
			Val:  hermesconst.HermesAndMM,
		}},
	})
}

func HermesTestCI(ctx context.Context, s *testing.State) {
	testMode, ok := s.Param().(string)
	if !ok {
		s.Fatal("Unable to read test mode")
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

	// Emulate Chrome's call that occurs during OOBE
	if testMode != hermesconst.HermesOnly {
		switchSlotIfMMTest(ctx, s, testMode)
		_, err = euicc.InstalledProfiles(ctx, true)
		if err != nil {
			s.Fatal("Failed to get installed profiles: ", err)
		}
	}

	const numProfiles = 2
	profiles := make([]*hermes.Profile, numProfiles)
	for i := 0; i < numProfiles; i++ {
		activationCode, cleanupFunc, err := stork.FetchStorkProfile(ctx, "")
		if err != nil {
			s.Fatal("Failed to fetch Stork profile: ", err)
		}
		defer cleanupFunc(ctx)
		s.Log("Fetched Stork profile with activation code: ", activationCode)
		profiles[i] = installAndEnableProfile(ctx, s, euicc, activationCode)
	}

	hermes.CheckNumInstalledProfiles(ctx, s, euicc, numProfiles)
	if testMode != hermesconst.HermesOnly {
		m, err := modemmanager.NewModem(ctx)
		if err != nil {
			s.Fatal("Failed to create Modem: ", err)
		}
		EID, err := m.GetEid(ctx)
		if err != nil {
			s.Fatal("Failed to read EID: ", err)
		}
		if EID == "" {
			s.Fatal("MM's EID is empty after profile enable: ")

		}
	}

	switchSlotIfMMTest(ctx, s, testMode)
	s.Log("Disabling profile ", profiles[numProfiles-1])
	if err := profiles[numProfiles-1].Call(ctx, hermesconst.ProfileMethodDisable).Err; err != nil {
		s.Fatal("Failed to disable profile: ", profiles[numProfiles-1])
	}
	if err := hermes.CheckProperty(ctx, profiles[numProfiles-1].DBusObject, hermesconst.ProfilePropertyState, int32(hermesconst.ProfileStateDisabled)); err != nil {
		s.Fatal("Failed to check profile state: ", err)
	}

	switchSlotIfMMTest(ctx, s, testMode)
	s.Log("Renaming profile ", profiles[0])
	if err := profiles[0].Call(ctx, hermesconst.ProfileMethodRename, "profile0").Err; err != nil {
		s.Fatal("Failed to rename profile: ", profiles[0])
	}
	hermes.CheckProperty(ctx, profiles[0].DBusObject, hermesconst.ProfilePropertyNickname, "profile0")

	switchSlotIfMMTest(ctx, s, testMode)
	s.Log("Uninstalling profile ", profiles[0])
	if err := euicc.Call(ctx, hermesconst.EuiccMethodUninstallProfile, profiles[0].DBusObject.ObjectPath()).Err; err != nil {
		s.Fatal("Failed to uninstall profile: ", profiles[0])
	}

	switchSlotIfMMTest(ctx, s, testMode)
	s.Log("Reset ", euicc)
	hermes.CheckNumInstalledProfiles(ctx, s, euicc, numProfiles-1)
	if err := euicc.Call(ctx, hermesconst.EuiccMethodResetMemory, 1).Err; err != nil {
		s.Fatal("Failed to reset test euicc: ", err)
	}

	switchSlotIfMMTest(ctx, s, testMode)
	hermes.CheckNumInstalledProfiles(ctx, s, euicc, 0)
	s.Log("Reset test euicc completed")
}

// installAndEnableProfile installs and enables a profile
func installAndEnableProfile(ctx context.Context, s *testing.State, euicc *hermes.EUICC, activationCode stork.ActivationCode) (p *hermes.Profile) {
	s.Logf("Installing profile %s", activationCode)
	response := euicc.Call(ctx, hermesconst.EuiccMethodInstallProfileFromActivationCode, activationCode, "")
	if response.Err != nil {
		s.Fatalf("Failed to install profile with %s: %s", activationCode, response.Err)
	}
	if len(response.Body) != 1 {
		s.Fatalf("InstallProfile resulted in incorrect response len: %d", len(response.Body))
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

func switchSlotIfMMTest(ctx context.Context, s *testing.State, testMode string) {
	if testMode == hermesconst.HermesAndMM {
		s.Log("Switching slots")
		if _, err := modemmanager.SwitchSlot(ctx); err != nil {
			s.Fatal("Failed to switch slots: ", err)
		}
	}
}
