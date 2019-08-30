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
		Func: EthernetStaticIP,
		Desc: "Test whether static IP configurations behave as they should between profile changes",
		Contacts: []string{
			"matthewmwang@chromium.org",
		},
		Attr: []string{"informational"},
	})
}

func EthernetStaticIP(ctx context.Context, s *testing.State) {
	const (
		filePath               = "/var/cache/shill/default.profile"
		testDefaultProfileName = "ethTestProfile"
		testUserProfileName    = "~chronos/test"
	)

	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer unlock()

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

	// Clean up custom services and test profiles on exit.
	defer func() {
		manager.PopProfile(ctx, testUserProfileName)
		manager.RemoveProfile(ctx, testUserProfileName)
		manager.PopProfile(ctx, testDefaultProfileName)
		manager.RemoveProfile(ctx, testDefaultProfileName)

		shill.SafeStop(ctx)
		os.Remove(filePath)
		shill.SafeStart(ctx)
	}()

	// Pop user profiles and push a temporary default profile on top
	s.Log("Popping all user profiles and pushing new default profile")
	if err = manager.PopAllUserProfiles(ctx); err != nil {
		s.Fatal("Failed to pop user profiles: ", err)
	}
	if _, err = manager.CreateProfile(ctx, testDefaultProfileName); err != nil {
		s.Fatal("Failed to create profile: ", err)
	}
	if _, err = manager.PushProfile(ctx, testDefaultProfileName); err != nil {
		s.Fatal("Failed to push profile: ", err)
	}

	// Find the Ethernet service and set the static IP
	s.Log("Setting static IP")
	servicePath, err := manager.FindMatchingService(ctx, map[string]interface{}{"Type": "ethernet"})
	if err != nil {
		s.Fatal("Unable to find service: ", err)
	}
	service, err := shill.NewService(ctx, servicePath)
	if err != nil {
		s.Fatal("Failed creating shill service proxy: ", err)
	}
	if err = service.SetProperty(ctx, "StaticIPConfig", map[string]interface{}{"Address": "10.9.8.2"}); err != nil {
		s.Fatal("Failed to set property: ", err)
	}

	// Test that static IP has been set
	s.Log("Finding service with set static IP")
	defaultProfileProps := map[string]interface{}{
		"Type":           "ethernet",
		"StaticIPConfig": map[string]interface{}{"Address": "10.9.8.2"},
	}
	if err = manager.WaitForServiceProperties(ctx, defaultProfileProps, 5); err != nil {
		s.Fatal("Unable to find service: ", err)
	}

	// Test that after profile push, property is still there
	s.Log("Pushing profile and checking that static IP has not changed")
	userProfilePath, err := manager.CreateProfile(ctx, testUserProfileName)
	if err != nil {
		s.Fatal("Failed to create profile: ", err)
	}
	if _, err = manager.PushProfile(ctx, testUserProfileName); err != nil {
		s.Fatal("Failed to push profile: ", err)
	}
	if err = manager.WaitForServiceProperties(ctx, defaultProfileProps, 5); err != nil {
		s.Fatal("Unable to find service: ", err)
	}

	// Configure service for user profile with different static IP
	s.Log("Configure different static IP for the new profile")
	userProfileProps := map[string]interface{}{
		"Type":           "ethernet",
		"StaticIPConfig": map[string]interface{}{"Address": "10.9.8.3"},
	}
	if _, err = manager.ConfigureServiceForProfile(ctx, userProfilePath, userProfileProps); err != nil {
		s.Fatal("Unable to configure service: ", err)
	}

	// Test that new static IP is there
	s.Log("Finding service with new static IP")
	if err = manager.WaitForServiceProperties(ctx, userProfileProps, 5); err != nil {
		s.Fatal("Unable to find service: ", err)
	}

	// Test that after profile pop, first static IP is there
	s.Log("Popping user profile and checking that static IP has reverted")
	if err = manager.PopProfile(ctx, testUserProfileName); err != nil {
		s.Fatal("Unable to pop profile: ", err)
	}
	if err = manager.WaitForServiceProperties(ctx, defaultProfileProps, 5); err != nil {
		s.Fatal("Unable to find service: ", err)
	}

	// Test that after push, second static IP is there again
	s.Log("Re-pushing the same user profile and checking that static IP")
	if _, err = manager.PushProfile(ctx, testUserProfileName); err != nil {
		s.Fatal("Failed to push profile: ", err)
	}
	if err = manager.WaitForServiceProperties(ctx, userProfileProps, 5); err != nil {
		s.Fatal("Unable to find service: ", err)
	}
}
