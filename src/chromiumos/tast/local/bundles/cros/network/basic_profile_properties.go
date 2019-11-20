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
	defaultProfilePath  = "/var/cache/shill/default.profile"
	testProfileName     = "test"
	ethernetEntryKey    = "ethernet_any"
	expextedProfilePath = "/profile/test"
)

func BasicProfileProperties(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Remove test profiles in case they already exist.
	manager.RemoveProfile(ctx, testProfileName)

	// Clean up custom test profiles on exit.
	defer func() {
		manager.PopProfile(ctx, testProfileName)
		manager.RemoveProfile(ctx, testProfileName)

		upstart.StopJob(ctx, "shill")
		os.Remove(defaultProfilePath)
		upstart.RestartJob(ctx, "shill")
	}()

	// Pop user profiles and push a temporary default profile on top.
	if err = manager.PopAllUserProfiles(ctx); err != nil {
		s.Fatal("Failed to pop all user's profiles: ", err)
	}
	if _, err = manager.CreateProfile(ctx, testProfileName); err != nil {
		s.Fatal("Failed to create the test profile: ", err)
	}
	if _, err = manager.PushProfile(ctx, testProfileName); err != nil {
		s.Fatal("Failed to push the test profile just created: ", err)
	}

	// Refreshes the Profiles in the cache memory after the previous changes.
	if _, err := manager.GetProperties(ctx); err != nil {
		s.Fatal("Failed to refresh Profiles in cache memory: ", err)
	}

	// Get current profiles.
	profiles, err := manager.GetProfiles(ctx)
	if err != nil {
		s.Fatal("Failed getting profiles: ", err)
	}
	s.Log("Got the test profiles: ", profiles)

	if len(profiles) <= 0 {
		s.Fatal("Profile Stack is empty")
	}
	// The last profile should be the one we just created.
	profilePath := profiles[len(profiles)-1]
	if profilePath != expextedProfilePath {
		s.Fatal("Failed getting profile from $profilePath: ", err)
	}

	newProfile, err := shill.NewProfile(ctx, profilePath)
	if err != nil {
		s.Fatal("Failed getting profile from $profilePath: ", err)
	}

	// Get the profile properties
	profProps := newProfile.Properties()
	s.Log("Profile Properties: ", profProps)

	// Get the Entries property of the profile.
	profPropsEntries, err := profProps.Get(shill.ProfilePropertyEntries)
	if err != nil {
		s.Fatal("Failed getting profile entries: ", err)
	}
	s.Log("Found Profile Properties Entries: ", profPropsEntries)

	profPropsEntriesArray, err := profProps.GetStrings(shill.ProfilePropertyEntries)
	if err != nil {
		s.Fatal("Failed converting the Profile Properties Entries interface to string array: ", err)
	}

	// Check if the "ethernet_any" object exists in the Entries of the profile.
	for _, p := range profPropsEntriesArray {
		if p == ethernetEntryKey {
			return
		}
	}
	s.Fatal("Missing ethernet entry from profile: ", ethernetEntryKey)
}
