// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"

	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultProfileServices,
		Desc: "Checks configured services persist across shill reboot",
		Contacts: []string{
			"briannorris@chromium.org",
			"chromeos-kernel-wifi@google.com", // WiFi team
			"oka@chromium.org",                // Tast port author
		},
		Attr: []string{"group:mainline"},
	})
}

func DefaultProfileServices(ctx context.Context, s *testing.State) {
	const (
		defaultProfile = "/var/cache/shill/default.profile"
		// ssid is a fake service name chosen unlikely to match any SSID present over-the-air.
		ssid = "org.chromium.DfltPrflSrvcsTest"
	)

	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer unlock()

	func() {
		// Stop shill temporarily and remove the default profile.
		if err := upstart.StopJob(ctx, "shill"); err != nil {
			s.Fatal("Failed stopping shill: ", err)
		}
		defer func() {
			if err := upstart.RestartJob(ctx, "shill"); err != nil {
				s.Fatal("Failed starting shill: ", err)
			}
		}()
		// TODO(oka): It's possible that the default profile has been
		// removed by the previous test, and this test has started
		// before the default profile is created by the previous test's
		// (re)starting of Shill. It's a confusing race condition, so
		// fix it by making sure that the default profile exsits here.
		if err := os.Remove(defaultProfile); err != nil && !os.IsNotExist(err) {
			s.Fatal("Failed removing default profile: ", err)
		}
	}()

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	if err := manager.PopAllUserProfiles(ctx); err != nil {
		s.Fatal("Failed to pop user profiles: ", err)
	}

	if err := manager.ConfigureService(ctx, map[string]interface{}{
		shill.ServicePropertyType:           "wifi",
		shill.ServicePropertyMode:           "managed",
		shill.ServicePropertySSID:           ssid,
		shill.ServicePropertyWiFiHiddenSSID: true,
		shill.ServicePropertySecurityClass:  "none",
	}); err != nil {
		s.Fatal("Failed to configure service: ", err)
	}

	// Restart shill to ensure that configurations persist across reboot.
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}

	manager, err = shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	if err := manager.PopAllUserProfiles(ctx); err != nil {
		s.Fatal("Failed to pop user profiles: ", err)
	}

	if _, err := manager.FindMatchingAnyService(ctx, map[string]interface{}{
		shill.ServicePropertyName: ssid,
	}); err != nil {
		s.Error("Network not found after restart: ", err)
	}
}
