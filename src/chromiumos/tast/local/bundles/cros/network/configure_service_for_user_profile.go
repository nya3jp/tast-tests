// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ConfigureServiceForUserProfile,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that we can configure a WiFi network for a user profile (guest or normal)",
		Contacts: []string{
			"stevenjb@chromium.org",
			"cros-networking@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "shill-wifi"},

		Params: []testing.Param{
			{
				Name:    "guest",
				Fixture: "chromeLoggedInGuest",
			},
			{
				Name:    "normal",
				Fixture: "chromeLoggedIn",
			},
		},
	})
}

// removeMatchingService helps clear out any similar pre-existing service.
func removeMatchingService(ctx context.Context, m *shill.Manager, props map[string]interface{}) error {
	service, err := m.FindMatchingService(ctx, props)
	if err != nil {
		if err.Error() == shillconst.ErrorMatchingServiceNotFound {
			return nil
		}
		return errors.Wrap(err, "error calling FindMatchingService")
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
		shillconst.ServicePropertyType:          shillconst.TypeWifi,
		shillconst.ServicePropertySecurityClass: shillconst.SecurityClassPSK,
		shillconst.ServicePropertySSID:          ssid,
		shillconst.ServicePropertyPassphrase:    "notarealpassword",
	}
	expectProps := map[string]interface{}{
		shillconst.ServicePropertyType:          shillconst.TypeWifi,
		shillconst.ServicePropertyName:          ssid,
		shillconst.ServicePropertySecurityClass: shillconst.SecurityClassPSK,
	}

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Ensure we got the right global + user profile set up on login. Because shill profiles are pushed
	// asynchronously from login, we wait.
	profilePath := func(ctx context.Context) dbus.ObjectPath {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		watcher, err := m.CreateWatcher(ctx)
		if err != nil {
			s.Fatal("Failed to create watcher: ", err)
		}
		defer watcher.Close(ctx)

		for {
			paths, err := m.ProfilePaths(ctx)
			if err != nil {
				s.Fatal("Failed to get profile paths: ", err)
			}
			if len(paths) > 2 {
				s.Fatalf("Too many profiles: got %d, want 2", len(paths))
			}
			if len(paths) == 2 {
				// Last profile is the user profile.
				return paths[len(paths)-1]
			}
			// Fewer than 2? We may still be pushing the user profile; let's wait.
			if _, err := watcher.WaitAll(ctx, shillconst.ManagerPropertyProfiles); err != nil {
				s.Fatal("Failed to wait for user profile: ", err)
			}
		}
	}(ctx)

	if err := removeMatchingService(ctx, m, expectProps); err != nil {
		s.Fatal("Failed to remove pre-existing WiFi service: ", err)
	}

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
