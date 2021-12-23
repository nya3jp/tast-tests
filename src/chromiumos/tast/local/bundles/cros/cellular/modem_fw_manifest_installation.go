// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/modemfwd"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModemFWManifestInstallation,
		Desc:         "Verifies that all modem FWs compatible with a device can be installed",
		Contacts:     []string{"andrewlassalle@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_sim_active", "cellular_unstable"},
		SoftwareDeps: []string{"modemfwd"},
		Timeout:      20 * time.Minute,
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

	// Get the device variant.
	dutVariant, err := crosconfig.Get(ctx, "/modem", "firmware-variant")
	if crosconfig.IsNotFound(err) {
		s.Log("Variant doesn't exist. Testing all variants in the DUT")
	} else if err != nil {
		s.Fatalf("Failed to execute cros_config: %s", err)
	}

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

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	// Ensure the test restores the modemfwd state.
	defer cleanUp(cleanupCtx, s)

	m, err := modemfwd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to modemfwd")
	}

	// Disable automatic modem update in modemfwd
	if err = ioutil.WriteFile(modemfwd.DisableAutoUpdatePref, []byte("1"), 0666); err != nil {
		s.Fatalf("Could not write to %s: %v", modemfwd.DisableAutoUpdatePref, err)
	}
	defer os.Remove(modemfwd.DisableAutoUpdatePref)

	for _, device := range manifest.Device {
		if dutVariant == device.Variant || (dutVariant == "" && deviceID == device.DeviceId) {
			for _, carrierFW := range device.CarrierFirmware {
				s.Logf("Force flashing for device %q and uuid %q", device.DeviceId, carrierFW.CarrierId[0])
				options := map[string]string{"uuid": carrierFW.CarrierId[0]}
				if err := m.ForceFlash(ctx, device.DeviceId, options); err != nil {
					s.Fatal("Failed to flash fw: ", err)
				}
			}

		}
		// TODO: flash a random variant/carrier to cover variants not present in the lab
	}

}

func cleanUp(ctx context.Context, s *testing.State) {
	job := "modemfwd"
	if !upstart.JobExists(ctx, job) {
		// Try to start the job before failing the test
		_ = upstart.StartJob(ctx, job)
		s.Fatal("modemfwd was not running")
	}
	if err := upstart.RestartJob(ctx, job); err != nil {
		s.Fatalf("Failed to restart job %q: %v", job, err)
	}
	s.Log("modemfwd restarted successfully")
}
