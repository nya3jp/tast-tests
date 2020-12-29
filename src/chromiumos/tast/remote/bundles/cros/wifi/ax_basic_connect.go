// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        AxBasicConnect,
		Desc:        "Tests our ability to connect to our third-party ax routers",
		Contacts:    []string{"hinton@google.com", "chromeos-platform-connectivity@google.com"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Attr:        []string{"group:wificell"},
	})
}

func AxBasicConnect(ctx context.Context, s *testing.State) {
	wifiutil.AxConnect(ctx, s, s.DUT(), "Velop-5G", "chromeos")
	wifiutil.AxConnect(ctx, s, s.DUT(), "Rapture-5G-1", "chromeos")
	wifiutil.AxConnect(ctx, s, s.DUT(), "Juplink-RX4-1500", "chromeos")
	wifiutil.AxConnect(ctx, s, s.DUT(), "NETGEAR69-5G", "chromeos")
}
