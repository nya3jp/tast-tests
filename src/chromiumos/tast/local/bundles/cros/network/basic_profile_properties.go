// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     BasicProfileProperties,
		Desc:     "Test that shill's DBus properties for profiles work",
		Contacts: []string{"arowa@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

const (
	defaultProfilePath     = "/var/cache/shill/default.profile"
	profileName            = "test"
	profilePropertyEntries = "Entries"
	ethernetEntryKey       = "ethernet_any"
)

func BasicProfileProperties(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Remove test profiles in case they already exist.
	manager.RemoveProfile(ctx, profileName)

	// Clean up custom test profiles on exit.
	defer func() {
		manager.PopProfile(ctx, profileName)
		manager.RemoveProfile(ctx, profileName)

		upstart.StopJob(ctx, "shill")
		os.Remove(defaultProfilePath)
		upstart.RestartJob(ctx, "shill")
	}()

	// Pop user profiles and push a temporary default profile on top.
	s.Log("Popping all user profiles and pushing new default profile")
	if err = manager.PopAllUserProfiles(ctx); err != nil {
		s.Fatal("Failed to pop user profiles: ", err)
	}
	if _, err = manager.CreateProfile(ctx, profileName); err != nil {
		s.Fatal("Failed to create profile: ", err)
	}
	if _, err = manager.PushProfile(ctx, profileName); err != nil {
		s.Fatal("Failed to push profile: ", err)
	}

	// Get current profiles.
	profiles, err := manager.GetProfiles(ctx)
	if err != nil {
		s.Fatal("Failed getting profiles: ", err)
	}
	s.Log("Got profiles ", profiles)

	if len(profiles) > 0 {
		// The last profile should be the one we just created.
		profilePath := profiles[len(profiles)-1]
		s.Log("Last Profile ", profilePath)

		newProfile, err := shill.NewProfile(ctx, profilePath)
		if err != nil {
			s.Fatal("Failed creating Profile: ", err)
		}

		// Get the profile properties
		profProps, err := newProfile.GetProperties(ctx)
		if err != nil {
			s.Fatal("Failed getting properties: ", err)
		}
		s.Log("Profile Properties: ", profProps)

		// Get the Entries property of the profile.
		profPropsEntries, err := profProps.Get(profilePropertyEntries)
		if err != nil {
			s.Fatal("Failed getting profile entries: ", err)
		}
		s.Log("Found Profile Properties Entries: ", profPropsEntries)

		// Check If the "ethernet_any" object exist in the Entries of the profile.
		found := false
		profPropsE := profPropsEntries.([]string)
		for i := 0; i < len(profPropsE); i++ {
			if profPropsE[i] == ethernetEntryKey {
				s.Log(profPropsE[i])
				found = true
			}
		}
		if !found {
			s.Fatal("Missing ethernet entry from profile: ", ethernetEntryKey)
		}
	}
}
