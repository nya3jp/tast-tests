// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func:         WifiScan,
		Desc:         "Verifies that shill and wpa_supplicant stack works as expected by sending a RequestScan command",
		Contacts:     []string{"deanliao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi"},
	})
}

func WifiScan(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	err := manager.RequestScan(ctx, shill.TechnologyWifi)
	if err != nil {
		s.Fatal("Failed to perform RequestScan on Wifi device: ", err)
	}
}
