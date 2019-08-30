// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultProfile,
		Desc: "Checks shill's default network profile",
		Contacts: []string{
			"kirtika@chromium.org", // Connectivity team
			"chromeos-kernel-wifi@google.com",
			"nya@chromium.org", // Tast port author
		},
		Attr: []string{"informational"},
	})
}

func DefaultProfile(ctx context.Context, s *testing.State) {
	const objectPath = dbus.ObjectPath("/profile/default")

	expectedSettings := []string{
		"CheckPortalList=ethernet,wifi,cellular",
		"IgnoredDNSSearchPaths=gateway.2wire.net",
		"LinkMonitorTechnologies=wifi",
	}

	// We lose connectivity briefly. Tell recover_duts not to worry.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer unlock()

	// Stop shill temporarily and remove the default profile.
	if err := shill.SafeStop(ctx); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}
	os.Remove(shill.DefaultProfile)
	if err := shill.SafeStart(ctx); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Wait for default profile creation.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// shill.SafeStart ensures the file exists.
		if _, err := os.Stat(shill.DefaultProfile); err != nil {
			return testing.PollBreak(err)
		}
		profiles, err := manager.GetProfiles(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		for _, p := range profiles {
			if p == objectPath {
				return nil
			}
		}
		return errors.Errorf("%q not found in the profiles %q", objectPath, profiles)
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Default profile didn't get ready: ", err)
	}

	// Read the default profile and check expected settings.
	b, err := ioutil.ReadFile(shill.DefaultProfile)
	if err != nil {
		s.Fatal("Failed reading the default profile: ", err)
	}
	ioutil.WriteFile(filepath.Join(s.OutDir(), "default.profile"), b, 0644)
	profile := string(b)

	for _, es := range expectedSettings {
		if !strings.Contains(profile, es) {
			s.Error("Expected setting not found: ", es)
		}
	}
}
