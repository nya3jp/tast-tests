// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"fmt"
	"strings"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WiFiWPAKey,
		Desc: "Checks WiFi WPA passphrase validation",
		Attr: []string{"informational"},
	})
}

func WiFiWPAKey(s *testing.State) {
	securities := []string{"rsn", "wpa", "psk"}

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

	ssidCounter := 0

	// check checks key is accepted (if good is true), or rejected (if good is false).
	check := func(security, key string, good bool) {
		s.Logf("Checking: %q (%s)", key, security)

		ssid := fmt.Sprintf("ssid%d", ssidCounter)
		ssidCounter++

		err := manager.ConfigureService(ctx, map[string]interface{}{
			"Type":       "wifi",
			"SSID":       ssid,
			"Security":   security,
			"Passphrase": key,
		})

		var success bool
		if err == nil {
			success = true
		} else if derr, ok := err.(dbus.Error); ok && derr.Name == "org.chromium.flimflam.Error.InvalidPassphrase" {
			success = false
		} else {
			s.Error(err)
			return
		}

		if success != good {
			s.Errorf("Got success=%v (want %v): %v", success, good, err)
		}
	}

	cases := []struct {
		key  string
		good bool
	}{
		{"good WPA key", true},
		{strings.Repeat("0123456789", 8)[:64], true},
		// Too long:
		{strings.Repeat("x123456789", 8)[:64], false},
		// Too short:
		{"x123456", false},
		// Too long (hex):
		{strings.Repeat("0123456789", 8)[:65], false},
	}
	for _, security := range securities {
		for _, c := range cases {
			check(security, c.key, c.good)
		}
	}
}
