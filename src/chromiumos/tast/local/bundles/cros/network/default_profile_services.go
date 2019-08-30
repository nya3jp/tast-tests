// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"

	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
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
		Attr: []string{"informational"},
	})
}

func DefaultProfileServices(ctx context.Context, s *testing.State) {
	// ssid is a fake service name chosen unlikely to match any SSID present over-the-air.
	const ssid = "org.chromium.DfltPrflSrvcsTest"

	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer unlock()

	func() {
		// Stop shill temporarily and remove the default profile.
		if err := shill.SafeStop(ctx); err != nil {
			s.Fatal("Failed stopping shill: ", err)
		}
		defer func() {
			if err := shill.SafeStart(ctx); err != nil {
				s.Fatal("Failed starting shill: ", err)
			}
		}()
		if err := os.Remove(shill.DefaultProfile); err != nil {
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
		"Type":            "wifi",
		"Mode":            "managed",
		"SSID":            ssid,
		"WiFi.HiddenSSID": true,
		"SecurityClass":   "none",
	}); err != nil {
		s.Fatal("Failed to configure service: ", err)
	}

	// Restart shill to ensure that configurations persist across reboot.
	if err := shill.SafeStop(ctx); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}
	if err := shill.SafeStart(ctx); err != nil {
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
		"Name": ssid,
	}); err != nil {
		s.Error("Network not found after restart: ", err)
	}
}
