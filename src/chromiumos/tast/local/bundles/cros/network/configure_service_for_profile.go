// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConfigureServiceForProfile,
		Desc: "Test ConfigureServiceForProfile D-Bus method",
		Contacts: []string{
			"matthewmwang@chromium.org",
		},
		Attr: []string{"informational"},
	})
}

func ConfigureServiceForProfile(ctx context.Context, s *testing.State) {
	const (
		filePath   = "/var/cache/shill/default.profile"
		objectPath = dbus.ObjectPath("/profile/default")
	)

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

	props := map[string]interface{}{
		"Type":                 "ethernet",
		"StaticIP.NameServers": "8.8.8.8",
	}
	_, err = manager.ConfigureServiceForProfile(ctx, objectPath, props)
	if err != nil {
		s.Fatal("Unable to configure service: ", err)
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

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		service, err := manager.FindMatchingService(ctx, props)
		if service != nil {
			return nil
		}
		return err
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: 100 * time.Millisecond}); err != nil {
		s.Fatal("service not found")
	}
}
