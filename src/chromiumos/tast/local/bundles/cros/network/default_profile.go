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

	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
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
		Attr: []string{"group:mainline"},
	})
}

func DefaultProfile(ctx context.Context, s *testing.State) {
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
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}
	os.Remove(shill.DefaultProfilePath)
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Wait for default profile creation.
	func() {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		isDefaultProfileReady := func() bool {
			if _, err := os.Stat(shill.DefaultProfilePath); err != nil {
				return false
			}

			paths, err := manager.ProfilePaths(ctx)
			if err != nil {
				s.Fatal("Failed getting profiles: ", err)
			}

			for _, p := range paths {
				if p == shill.DefaultProfileObjectPath {
					return true
				}
			}
			return false
		}

		for !isDefaultProfileReady() {
			if err := testing.Sleep(ctx, 100*time.Millisecond); err != nil {
				s.Fatal("Timed out waiting for the default profile to get ready: ", err)
			}
		}
	}()

	// Read the default profile and check expected settings.
	b, err := ioutil.ReadFile(shill.DefaultProfilePath)
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
