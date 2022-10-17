// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemfwd"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModemFWManifestInstallation,
		Desc:         "Verifies that all modem FWs compatible with a device can be installed",
		Contacts:     []string{"andrewlassalle@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_modem_fw"},
		Fixture:      "cellular",
		SoftwareDeps: []string{"modemfwd"},
		Timeout:      12 * time.Minute,
	})
}

// ModemFWManifestInstallation Test
func ModemFWManifestInstallation(ctx context.Context, s *testing.State) {
	fileExists := func(file string) bool {
		_, err := os.Stat(file)
		return !os.IsNotExist(err)
	}

	if !fileExists(cellular.GetModemFirmwareManifestPath()) {
		s.Fatal("Cannot find ", cellular.GetModemFirmwareManifestPath())
	}

	manifest, err := cellular.ParseModemFirmwareManifest(ctx)
	if err != nil {
		s.Fatal("Failed to parse the firmware manifest: ", err)
	}

	// Ignore the error since older boards didn't always use variants and this
	// test does not require a variant to succeed.
	dutVariant, _ := cellular.GetDeviceVariant(ctx)

	// Find the USB device ID of the modem in this variant.
	deviceID := ""
	for _, device := range manifest.Device {
		if dutVariant == device.Variant || dutVariant == "" {
			deviceID = device.DeviceId
			break
		}
	}
	if deviceID == "" {
		s.Fatal("Failed to find the USB device ID on the manifest")
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Try to get the carrier_id of the current SIM card in the DUT, so we can flash the corresponding
	// FW after all the other ones. This will automatically leave the DUT with the correct FW.
	uuid, _, err := helper.GetHomeProviderFromShill(ctx)
	if err != nil {
		s.Log("Failed to get carrier ID from shill. Continuing anyway: ", err)
	}

	deferCleanUp, err := modemfwd.DisableAutoUpdate(ctx)
	if err != nil {
		s.Fatal("Failed to set DisableAutoUpdatePref: ", err)
	}
	defer deferCleanUp()

	// modemfwd is initially stopped in the fixture SetUp
	if err = modemfwd.StartAndWaitForQuiescence(ctx); err != nil {
		s.Fatal("modemfwd failed during initialization: ", err)
	}
	s.Log("modemfwd has started successfully")

	m, err := modemfwd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to modemfwd")
	}

	var carrierIDs = []string{}
	for _, device := range manifest.Device {
		if dutVariant == device.Variant || (dutVariant == "" && deviceID == device.DeviceId) {
			for _, carrierFW := range device.CarrierFirmware {
				for _, carrierID := range carrierFW.CarrierId {
					if uuid == carrierID {
						continue
					}
				}
				if len(carrierFW.CarrierId) > 0 {
					carrierIDs = append(carrierIDs, carrierFW.CarrierId[0])
				}
			}
		}
	}
	if uuid != "" {
		carrierIDs = append(carrierIDs, uuid)
	}

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	// Ensure the test restores the modemfwd state.
	defer cleanUp(cleanupCtx, s)

	// Use a combination of minimum number of retries and time. Sometimes failures take a long time,
	// and a single retry is enough, but when the modem is not ready for an update, the retries will
	// fail very quickly.
	const minNumberOfRetries = 2
	const minTimeForRetries = 40 * time.Second
	// Ensure we do a full flash at least once.
	useModemsFwInfo := false
	for _, carrierID := range carrierIDs {
		// Try each FW twice to reduce flakiness due to random failure.
		for i := 1; ; i++ {
			startTime := time.Now()
			const usbPrefix = "usb:"
			if strings.HasPrefix(deviceID, usbPrefix) {
				if err := modemfwd.WaitForUsbDevice(ctx, strings.TrimPrefix(deviceID, usbPrefix), 40*time.Second); err != nil {
					s.Fatal("Failed to flash fw: ", err)
				}
			}

			s.Logf("Force flashing for device %q and uuid %q. Retry %d", deviceID, carrierID, i)
			options := map[string]interface{}{"carrier_uuid": carrierID, "use_modems_fw_info": useModemsFwInfo}
			useModemsFwInfo = true
			if err := m.ForceFlash(ctx, deviceID, options); err != nil {
				if i < minNumberOfRetries || time.Now().Sub(startTime) < minTimeForRetries {
					s.Logf("Failed to flash fw: %q. Retrying same FW", err)
				} else {
					s.Fatal("Failed to flash fw: ", err)
				}
			} else {
				break
			}
		}

	}
}

// cleanUp ensures that modemfwd is fully restarted so if a FW needs to be installed on start,
// it will happen during this test.
func cleanUp(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "cleanUp")
	defer st.End()
	if err := upstart.StopJob(ctx, modemfwd.JobName); err != nil {
		s.Fatal("Failed to stop modemfwd: ", err)
	}
	if err := modemfwd.StartAndWaitForQuiescence(ctx); err != nil {
		s.Fatal("modemfwd failed during initialization: ", err)
	}
	// Stop the job one last time since the fixture expects modemfwd to be stopped.
	if err := upstart.StopJob(ctx, modemfwd.JobName); err != nil {
		s.Fatal("Failed to stop modemfwd: ", err)
	}
	s.Log("modemfwd has started successfully")

	// b/253685780: This test flashes a new FW image, and on vilboz, the FW always gets reset
	// to 'fast.t-mobile.com' even if the SIM card is a verizon SIM card.
	cellular.CheckIfVilbozVerizonAndFixAttachAPN(ctx)
}
