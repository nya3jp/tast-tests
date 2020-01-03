// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WifiScan,
		Desc:         "Checks the WiFi device is powered and can perform shill.RequestScan",
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

	iface, err := shill.GetWifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to get a WiFi interface: ", err)
	}
	dev, err := manager.GetDevice(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get a WiFi device: ", err)
	}

	powered, err := dev.Properties().GetBool("Powered")
	if err != nil {
		s.Fatal("Failed to get 'Powered' property in WiFi device: ", err)
	} else if !powered {
		s.Fatalf("WiFi device %s is not powered", iface)
	}

	err = manager.RequestScan(ctx, shill.TechnologyWifi)
	if err != nil {
		s.Fatal("Failed to perform RequestScan on Wifi device: ", err)
	}
}
