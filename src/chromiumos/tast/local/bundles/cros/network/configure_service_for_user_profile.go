// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst/security"
	"chromiumos/tast/common/shillconst/svcprop"
	"chromiumos/tast/common/shillconst/techtype"
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
		SoftwareDeps: []string{"chrome", "shill-wifi"},

		Params: []testing.Param{
			{
				Name: "guest",
				// It's a bit odd to create a single-use precondition; this is just for symmetry with
				// the chrome.LoggedIn() usage, where we don't need a separate login instance; we can
				// share with other tests.
				Pre: chrome.NewPrecondition("guest_logged_in", chrome.GuestLogin()),
			},
			{
				Name: "normal",
				Pre:  chrome.LoggedIn(),
			},
		},
	})
}

// removeMatchingService helps clear out any similar pre-existing service.
func removeMatchingService(ctx context.Context, m *shill.Manager, props map[string]interface{}) error {
	service, err := m.FindMatchingService(ctx, props)
	if err != nil {
		// No match is not a problem.
		return nil
	}
	testing.ContextLog(ctx, "Deleting existing service: ", service)
	return service.Remove(ctx)
}

func ConfigureServiceForUserProfile(ctx context.Context, s *testing.State) {
	const (
		// ssid is a fake service name chosen unlikely to match any SSID present over-the-air.
		ssid = "org.chromium.DfltPrflSrvcsTest"
	)
	props := map[string]interface{}{
		svcprop.Type:          techtype.Wifi,
		svcprop.SecurityClass: security.PSK,
		svcprop.SSID:          ssid,
		svcprop.Passphrase:    "notarealpassword",
	}
	expectProps := map[string]interface{}{
		svcprop.Type:          techtype.Wifi,
		svcprop.Name:          ssid,
		svcprop.SecurityClass: security.PSK,
	}

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	if err := removeMatchingService(ctx, m, expectProps); err != nil {
		s.Fatal("Failed to remove pre-existing WiFi service: ", err)
	}

	// Ensure we got the right global + user profile set up on login.
	paths, err := m.ProfilePaths(ctx)
	if err != nil {
		s.Fatal("Failed to get profile paths: ", err)
	}
	if len(paths) != 2 {
		s.Fatalf("Unexpected number of profiles: got %d, want 2", len(paths))
	}
	// Last profile is the user profile.
	profilePath := paths[len(paths)-1]

	s.Log("Configuring WiFi network with props ", props)
	if _, err := m.ConfigureServiceForProfile(ctx, profilePath, props); err != nil {
		s.Fatal("Failed to configure service: ", err)
	}

	if _, err := m.FindMatchingService(ctx, expectProps); err != nil {
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

		if err := upstart.RestartJob(ctx, "shill"); err != nil {
			s.Fatal("Failed restarting shill: ", err)
		}
	}()

	m, err = shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	if _, err := m.WaitForServiceProperties(ctx, expectProps, 5*time.Second); err != nil {
		s.Error("Network not found after restart: ", err)
	}
}
