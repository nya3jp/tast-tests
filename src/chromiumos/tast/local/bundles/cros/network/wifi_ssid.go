// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"strings"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WiFiSSID,
		Desc: "Checks WiFi SSID validation",
		Attr: []string{"informational"},
	})
}

func WiFiSSID(s *testing.State) {
	ctx := s.Context()

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	closer, err := manager.TemporaryProfile(ctx)
	if err != nil {
		s.Fatal("Failed creating a temporary profile", err)
	}
	defer closer(ctx)

	// check checks ssid is accepted (if good is true), or rejected (if good is false).
	check := func(ssid string, good bool) {
		s.Logf("Checking: %q", ssid)

		err := manager.ConfigureService(ctx, map[string]interface{}{
			"Type":          "wifi",
			"SSID":          ssid,
			"SecurityClass": "none",
		})

		var success bool
		if err == nil {
			success = true
		} else if derr, ok := err.(dbus.Error); ok && derr.Name == "org.chromium.flimflam.Error.InvalidNetworkName" {
			success = false
		} else {
			s.Error(err)
			return
		}

		if success != good {
			s.Errorf("Got success=%v (want %v): %v", success, good, err)
		}
	}

	check("good SSID", true)
	check("😺🐾", true)
	check(strings.Repeat("x", 33), false)
	check("", false)
}
