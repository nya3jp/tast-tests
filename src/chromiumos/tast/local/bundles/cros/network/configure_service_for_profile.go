// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConfigureServiceForProfile,
		Desc: "Test ConfigureServiceForProfile D-Bus method",
		Contacts: []string{
			"matthewmwang@chromium.org",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func ConfigureServiceForProfile(ctx context.Context, s *testing.State) {
	const (
		filePath   = shill.DefaultProfilePath
		objectPath = shill.DefaultProfileObjectPath
	)

	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer unlock()

	// Stop shill temporarily and remove the default profile.
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}
	os.Remove(filePath)
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Clean up custom services on exit.
	defer func() {
		upstart.StopJob(ctx, "shill")
		os.Remove(filePath)
		upstart.RestartJob(ctx, "shill")
	}()

	if err = manager.PopAllUserProfiles(ctx); err != nil {
		s.Fatal("Failed to pop user profiles: ", err)
	}

	props := map[string]interface{}{
		shill.ServicePropertyType: "ethernet",
		shill.ServicePropertyStaticIPConfig: map[string]interface{}{
			shill.IPConfigPropertyNameServers: []string{"8.8.8.8"},
		},
	}
	_, err = manager.ConfigureServiceForProfile(ctx, objectPath, props)
	if err != nil {
		s.Fatal("Unable to configure service: ", err)
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

	if _, err := manager.WaitForServiceProperties(ctx, props, 8*time.Second); err != nil {
		s.Fatal("Service not found: ", err)
	}
}
