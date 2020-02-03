// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/shill"
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
	testProfileName     = "test"
	ethernetEntryKey    = "ethernet_any"
	expectedProfilePath = "/profile/test"
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
	}()

	// Pop user profiles and push a temporary default profile on top.
	if err = manager.PopAllUserProfiles(ctx); err != nil {
		s.Fatal("Failed to pop all user profiles: ", err)
	}
	if _, err = manager.CreateProfile(ctx, testProfileName); err != nil {
		s.Fatal("Failed to create the test profile: ", err)
	}
	if _, err = manager.PushProfile(ctx, testProfileName); err != nil {
		s.Fatal("Failed to push the test profile just created: ", err)
	}

	// Get current profiles.
	profiles, err := manager.ProfilePaths(ctx)
	if err != nil {
		s.Fatal("Failed getting profiles: ", err)
	}

	if len(profiles) == 0 {
		s.Fatal("Profile stack is empty")
	}

	// The last profile should be the one we just created.
	profilePath := profiles[len(profiles)-1]
	if profilePath != expectedProfilePath {
		s.Fatalf("Found unexpected profile path: got %v, want %v", profilePath, expectedProfilePath)
	}

	newProfile, err := shill.NewProfile(ctx, profilePath)
	if err != nil {
		s.Fatalf("Failed getting profile from %v: %v", profilePath, err)
	}

	// Get the profile properties.
	profProps, err := newProfile.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed getting profile properties: ", err)
	}

	// Get the Entries property of the profile.
	profPropsEntries, err := profProps.GetStrings(shill.ProfilePropertyEntries)
	if err != nil {
		s.Fatal("Failed getting profile entries: ", err)
	}

	// Check if the "ethernet_any" object exists in the Entries of the profile.
	for _, p := range profPropsEntries {
		if p == ethernetEntryKey {
			return
		}
	}

	// Expected Ethernet entry is missing, fail the test with current profile property entries for debugging.
	s.Errorf("Missing Ethernet entry (%s) in test profile property: %v", ethernetEntryKey, profPropsEntries)
}
