// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularInhibited,
		Desc: "Tests the Shill Device.Inhibited property",
		Contacts: []string{
			"stevenjb@google.com",
			"cros-network-health@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr: []string{"group:cellular"},
	})
}

// ShillCellularInhibited Test
func ShillCellularInhibited(ctx context.Context, s *testing.State) {
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create Helper: ", err)
	}

	// Disable AutoConnect so that enable does not connect.
	// This also waits for a Cellular Service to be available.
	ctxForAutoConnectCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cellular.AutoConnectCleanupTime)
	defer cancel()
	if wasAutoConnect, err := helper.SetServiceAutoConnect(ctx, false); err != nil {
		s.Fatal("Failed to disable AutoConnect: ", err)
	} else if wasAutoConnect {
		defer func(ctx context.Context) {
			if _, err := helper.SetServiceAutoConnect(ctx, true); err != nil {
				s.Fatal("Failed to enable AutoConnect: ", err)
			}
		}(ctxForAutoConnectCleanUp)
	}

	for i := 0; i < 3; i++ {
		s.Logf("Inhibit Cellular Modem: %d", i)
		if err := helper.SetDeviceProperty(ctx, shillconst.DevicePropertyInhibited, true); err != nil {
			s.Fatal("Unable to set Device.Inhibited=true: ", err)
		}
		testing.Sleep(ctx, 5*time.Second)
		s.Logf("Uninhibit Cellular Modem: %d", i)
		if err := helper.SetDeviceProperty(ctx, shillconst.DevicePropertyInhibited, false); err != nil {
			s.Fatal("Unable to set Device.Inhibited=false: ", err)
		}
		testing.Sleep(ctx, 5*time.Second)
	}

	s.Log("Inhibit Cellular Modem")
	if err := helper.SetDeviceProperty(ctx, shillconst.DevicePropertyInhibited, true); err != nil {
		s.Fatal("Unable to set Device.Inhibited=true: ", err)
	}
	// Sleep to simulate performing a Hermes operation.
	testing.Sleep(ctx, 1*time.Second)

	s.Log("Uninhibit Cellular Modem")
	if err := helper.SetDeviceProperty(ctx, shillconst.DevicePropertyInhibited, false); err != nil {
		s.Fatal("Unable to set Device.Inhibited=false: ", err)
	}

	// Make sure that Connect succeeds after inhibit / uninhibit.
	// Note: It may take a long time for a Service to appear.
	s.Log("Verify Cellular Service (this may take up to 60 seconds)")
	if service, err := helper.FindServiceForDevice(ctx, 60*time.Second); err != nil {
		s.Fatal("No Cellular Service after uninhibit: ", err)
	} else if err := service.Connect(ctx); err != nil {
		s.Fatal("Unable to connect to service after uninhibit: ", err)
	}

	s.Log("Inhibit Cellular Modem while connected")
	if err := helper.SetDeviceProperty(ctx, shillconst.DevicePropertyInhibited, true); err != nil {
		s.Fatal("Unable to set Device.Inhibited=true: ", err)
	}

	s.Log("Verify no Cellular Service")
	if _, err = helper.FindServiceForDevice(ctx, 1*time.Second); err == nil {
		s.Fatal("Cellular Service found after inhibit")
	} else if !strings.Contains(err.Error(), "Matching service was not found") {
		s.Fatal("Error finding Cellular Service after inhibit: ", err)
	}

	s.Log("Uninhibit Cellular Modem")
	if err := helper.SetDeviceProperty(ctx, shillconst.DevicePropertyInhibited, false); err != nil {
		s.Fatal("Unable to set Device.Inhibited=false: ", err)
	}

	// Make sure that Connect succeeds after a second uninhibit.
	// Note: It may take a long time for a Service to appear.
	s.Log("Verify Cellular Service (this may take up to 60 seconds)")
	if service, err := helper.FindServiceForDevice(ctx, 60*time.Second); err != nil {
		s.Fatal("No Cellular Service after uninhibit: ", err)
	} else if err := service.Connect(ctx); err != nil {
		s.Fatal("Unable to connect to service after uninhibit: ", err)
	}
}
