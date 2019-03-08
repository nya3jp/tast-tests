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
		Desc: "Test ConfigureServiceForProfile dbus method",
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

	func() {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		foundMatchingService := func() bool {
			service, _ := manager.FindMatchingService(ctx, props)
			return service != nil
		}

		for !foundMatchingService() {
			select {
			case <-time.After(100 * time.Millisecond):
			case <-ctx.Done():
				s.Fatal("Service not found")
			}
		}
	}()
}
