// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConfigureServiceForUserProfile,
		Desc: "Checks that we can configure a WiFi network for a user profile (guest or normal)",
		Contacts: []string{
			"briannorris@chromium.org",
			"chromeos-platform-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},

		Params: []testing.Param{
			{
				Name: "guest",
				Val:  []chrome.Option{chrome.GuestLogin()},
			},
			{
				Name: "normal",
				Val:  []chrome.Option{},
			},
		},
	})
}

func clearWiFiProfileEntries(ctx context.Context, m *shill.Manager) error {
	profiles, err := m.Profiles(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get profiles")
	}
	for _, profile := range profiles {
		props, err := profile.GetProperties(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get properties from profile object")
		}
		entryIDs, err := props.GetStrings(shill.ProfilePropertyEntries)
		if err != nil {
			return errors.Wrapf(err, "failed to get property %s from profile object", shill.ProfilePropertyEntries)
		}
		for _, entryID := range entryIDs {
			entry, err := profile.GetEntry(ctx, entryID)
			if err != nil {
				return errors.Wrapf(err, "failed to get entry %s", entryID)
			}
			if entry[shill.ProfileEntryPropertyType] != shill.TypeWifi {
				continue
			}
			testing.ContextLogf(ctx, "Deleting existing WiFi entry %v, in profile %v", entryID, profile)
			if err := profile.DeleteEntry(ctx, entryID); err != nil {
				return errors.Wrapf(err, "failed to delete entry %s", entryID)
			}
		}
	}
	return nil
}

func ConfigureServiceForUserProfile(ctx context.Context, s *testing.State) {
	const (
		// ssid is a fake service name chosen unlikely to match any SSID present over-the-air.
		ssid = "org.chromium.DfltPrflSrvcsTest"
	)
	props := map[string]interface{}{
		shill.ServicePropertyType:          shill.TypeWifi,
		shill.ServicePropertySecurityClass: shill.SecurityPSK,
		shill.ServicePropertySSID:          ssid,
		shill.ServicePropertyPassphrase:    "notarealpassword",
	}
	expectProps := map[string]interface{}{
		shill.ServicePropertyType:          shill.TypeWifi,
		shill.ServicePropertyName:          ssid,
		shill.ServicePropertySecurityClass: shill.SecurityPSK,
	}

	crOpts := s.Param().([]chrome.Option)

	cr, err := chrome.New(ctx, crOpts...)
	if err != nil {
		s.Fatal("Failed to log into Chrome: ", err)
	}
	defer cr.Close(ctx)

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	if err := clearWiFiProfileEntries(ctx, m); err != nil {
		s.Fatal("Failed to clear WiFi entries: ", err)
	}
	paths, err := m.ProfilePaths(ctx)
	if err != nil {
		s.Fatal("Failed to get profile paths: ", err)
	}
	if len(paths) != 2 {
		s.Fatalf("Unexpected number of profiles: got %d want 2", len(paths))
	}
	// Last profile is the user profile.
	profilePath := paths[len(paths)-1]

	s.Log("Configuring WiFi network with props ", props)
	if _, err := m.ConfigureServiceForProfile(ctx, profilePath, props); err != nil {
		s.Fatal("Failed to configure service: ", err)
	}

	if _, err := m.FindAnyMatchingService(ctx, expectProps); err != nil {
		s.Fatal("Configured network not found: ", err)
	}

	s.Log("Restarting Shill to ensure persistence")
	func() {
		// We lose connectivity when restarting Shill, and if that
		// races with the recover_duts network-recovery hooks, it may
		// interrupt us.
		unlock, err := network.LockCheckNetworkHook(ctx)
		if err != nil {
			s.Fatal("Failed to lock the check network hook: ", err)
		}
		defer unlock()

		if err := upstart.StopJob(ctx, "shill"); err != nil {
			s.Fatal("Failed stopping shill: ", err)
		}
		if err := upstart.RestartJob(ctx, "shill"); err != nil {
			s.Fatal("Failed starting shill: ", err)
		}
	}()

	m, err = shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	if _, err := m.WaitForAnyServiceProperties(ctx, expectProps, 5*time.Second); err != nil {
		s.Error("Network not found after restart: ", err)
	}
}
