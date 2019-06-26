// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

import (
	"context"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WifiConnectivity,
		Desc:     "Ensures that WiFi chip can recover from suspend resume properly",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}
func WifiConnectivity(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	props := map[string]interface{}{
		"Type":          "wifi",
		"Name":          "REDACTEDAPNAME",
		"SecurityClass": "none",
		"Mode":          "managed",
	}

	err = manager.DisconnectFromWifiNetwork(ctx, props)
	if err != nil {
		s.Fatal("Could not ConnectToWifiNetwork: ", err)
	}
	err = manager.ConnectToWifiNetwork(ctx, props)
	if err != nil {
		s.Fatal("Could not ConnectToWifiNetwork: ", err)
	}
	err = manager.DisconnectFromWifiNetwork(ctx, props)
	if err != nil {
		s.Fatal("Could not ConnectToWifiNetwork: ", err)
	}
}
