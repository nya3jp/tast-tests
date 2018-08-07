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
		Func: WiFiWEPKey,
		Desc: "Checks WiFi WEP key validation",
		Attr: []string{"informational"},
	})
}

func WiFiWEPKey(s *testing.State) {
	var (
		longHexString = strings.Repeat("0123456789abcdef", 8)
		badPrefixes   = []string{"4", "001", "-22", "a"}
	)

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
	check := func(key string, good bool) {
		s.Logf("Checking: %q", key)

		ssid := fmt.Sprintf("ssid%d", ssidCounter)
		ssidCounter++

		err := manager.ConfigureService(ctx, map[string]interface{}{
			"Type":          "wifi",
			"SSID":          ssid,
			"SecurityClass": "wep",
			"Passphrase":    key,
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

	for index := 0; index < 4; index++ {
		for length := 0; length < 30; length++ {
			keys := []string{
				fmt.Sprintf("%d:%s", index, longHexString[:length]),
				fmt.Sprintf("%d:0x%s", index, longHexString[:length]),
			}
			for _, key := range keys {
				good :=
					// Valid hex keys:
					length == 10 || length == 26 ||
						// Valid ASCII keys:
						len(key) == 5 || len(key) == 13 ||
						// Valid ASCII keys with index prefix:
						len(key) == 7 || len(key) == 15
				check(key, good)
			}
		}
	}

	for _, validKey := range []string{longHexString[:10], longHexString[:26], "wep40", "wep104is13len"} {
		for _, badPrefix := range badPrefixes {
			key := fmt.Sprintf("%s:%s", badPrefix, validKey)
			check(key, false)
		}
	}
}
