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

	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultProfile,
		Desc: "Checks shill's default network profile",
		Contacts: []string{
			"kirtika@chromium.org", // Connectivity team
			"nya@chromium.org",     // Tast port author
		},
	})
}

func DefaultProfile(ctx context.Context, s *testing.State) {
	const (
		filePath   = "/var/cache/shill/default.profile"
		objectPath = dbus.ObjectPath("/profile/default")
	)

	expectedSettings := []string{
		"CheckPortalList=ethernet,wifi,cellular",
		"IgnoredDNSSearchPaths=gateway.2wire.net",
		"LinkMonitorTechnologies=wifi",
	}

	// Stop shill temporarily and remove the default profile.
	if err := shill.SafeStop(ctx); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}
	os.Remove(filePath)
	if err := shill.SafeStart(ctx); err != nil {
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
			if _, err := os.Stat(filePath); err != nil {
				return false
			}

			profiles, err := manager.GetProfiles(ctx)
			if err != nil {
				s.Fatal("Failed getting profiles: ", err)
			}

			for _, p := range profiles {
				if p == objectPath {
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
	b, err := ioutil.ReadFile(filePath)
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
