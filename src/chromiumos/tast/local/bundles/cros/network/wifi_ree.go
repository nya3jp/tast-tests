// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

import (
	"context"
	// "fmt"
	// "strings"
	// "time"

	// "chromiumos/tast/errors"
	// "chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WifiRee,
		Desc:     "Ensures that WiFi chip can recover from suspend resume properly",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}
func WifiRee(ctx context.Context, s *testing.State) {
	// Hook into shill service.
	manager, err := shill.NewManager(ctx)

	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	out, err := manager.ConnectToWifiNetwork(ctx)
	if err != nil {
		s.Fatal("Could not ConnectToWifiNetwork", err)
	}
	if !out {
		s.Fatal("ConnectToWifiNetwork Failed")
	}

}
