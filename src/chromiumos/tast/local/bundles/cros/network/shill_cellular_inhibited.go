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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularInhibited,
		Desc: "Tests the Shill Device.Inhibited property",
		Contacts: []string{
			"stevenjb@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular"},
		Timeout: 10 * time.Minute,
	})
}

func verifyNotConnected(ctx context.Context, helper *cellular.Helper) error {
	props := map[string]interface{}{
		shillconst.ServicePropertyIsConnected: true,
		shillconst.ServicePropertyType:        shillconst.TypeCellular,
	}
	if s, err := helper.Manager.FindMatchingService(ctx, props); err == nil {
		return errors.Errorf("unexpected connected service found: %s", s.String())
	} else if !strings.Contains(err.Error(), shillconst.ErrorMatchingServiceNotFound) {
		return errors.Wrap(err, "error waiting for service")
	}
	return nil
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

	const timeout = 1 * time.Minute
	// Connect can take a long time after an un-inhibit while the Modem starts.
	const connectTimeout = 5 * time.Minute
	s.Log("Inhibit Cellular Modem")
	if err := helper.SetDeviceProperty(ctx, shillconst.DevicePropertyInhibited, true, timeout); err != nil {
		s.Fatal("Unable to set Device.Inhibited to true: ", err)
	}
	s.Log("Verify not connected to Cellular after Inhibit")
	if err := verifyNotConnected(ctx, helper); err != nil {
		s.Fatal("Cellular connected after inhibit: ", err)
	}
	s.Log("Uninhibit Cellular Modem")
	if err := helper.SetDeviceProperty(ctx, shillconst.DevicePropertyInhibited, false, timeout); err != nil {
		s.Fatal("Unable to set Device.Inhibited to false: ", err)
	}

	// Wait for Scanning to be false.
	if err := helper.Device.WaitForProperty(ctx, shillconst.DevicePropertyScanning, false, timeout); err != nil {
		s.Fatal("Scanning still true after Inhibit set to false: ", err)
	}

	// Make sure that Connect succeeds after inhibit / uninhibit.
	// Note: It may take a long time for a Service to appear.
	s.Logf("Verify Cellular Service and Connect (this may take up to %v)", timeout)
	if service, err := helper.FindServiceForDeviceWithTimeout(ctx, timeout); err != nil {
		s.Fatal("No Cellular Service after uninhibit: ", err)
	} else if err = helper.ConnectToServiceWithTimeout(ctx, service, connectTimeout); err != nil {
		s.Fatal("Error connecting to service after uninhibit: ", err)
	}

	s.Log("Inhibit Cellular Modem while connected")
	if err := helper.SetDeviceProperty(ctx, shillconst.DevicePropertyInhibited, true, timeout); err != nil {
		s.Fatal("Unable to set Device.Inhibited=true: ", err)
	}

	s.Log("Verify not connected to Cellular after Inhibit while connected")
	if err := verifyNotConnected(ctx, helper); err != nil {
		s.Fatal("Cellular connected after inhibit: ", err)
	}

	s.Log("Uninhibit Cellular Modem")
	if err := helper.SetDeviceProperty(ctx, shillconst.DevicePropertyInhibited, false, timeout); err != nil {
		s.Fatal("Unable to set Device.Inhibited=false: ", err)
	}

	// Wait for Scanning to be false.
	if err := helper.Device.WaitForProperty(ctx, shillconst.DevicePropertyScanning, false, timeout); err != nil {
		s.Fatal("Scanning still true after Inhibit set to false: ", err)
	}

	// Make sure that Connect succeeds after a second uninhibit.
	// Note: It may take a long time for a Service to appear.
	s.Logf("Verify Cellular Service (this may take up to %v)", timeout)
	if service, err := helper.FindServiceForDeviceWithTimeout(ctx, timeout); err != nil {
		s.Fatal("No Cellular Service after uninhibit: ", err)
	} else if err := helper.ConnectToServiceWithTimeout(ctx, service, connectTimeout); err != nil {
		s.Fatal("Unable to connect to service after uninhibit: ", err)
	}
}
