// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/crosconfig"
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
		Attr:         []string{"group:cellular", "cellular_sim_active", "cellular_unstable"},
		Fixture:      "cellular",
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

	// Try to get the carrier_id of the current SIM card in the DUT, so we can flash the corresponding
	// FW after all the other ones. This will automatically leave the DUT with the correct FW.
	uuid, err := getUUIDFromShill(ctx)
	if err != nil {
		s.Log("Failed to get carrier ID from shill. Continuing anyway: ", err)
	}

	// Disable automatic modem update in modemfwd
	if err = ioutil.WriteFile(modemfwd.DisableAutoUpdatePref, []byte("1"), 0666); err != nil {
		s.Fatalf("Could not write to %s: %v", modemfwd.DisableAutoUpdatePref, err)
	}
	defer os.Remove(modemfwd.DisableAutoUpdatePref)

	// modemfwd is initially stopped in the fixture SetUp

	if err = modemfwd.StartAndWaitForQuiescence(ctx); err != nil {
		s.Fatal("modemfwd failed during initialization: ", err)
	}
	s.Log("modemfwd has started successfully")

	m, err := modemfwd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to modemfwd")
	}

	// TODO: flash a random variant/carrier to cover variants not present in the lab
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

	for _, carrierID := range carrierIDs {
		s.Logf("Force flashing for device %q and uuid %q", deviceID, carrierID)
		options := map[string]string{"carrier_uuid": carrierID}
		if err := m.ForceFlash(ctx, deviceID, options); err != nil {
			s.Fatal("Failed to flash fw: ", err)
		}
	}
}

// cleanUp ensures that modemfwd is fully restarted so if a FW needs to be installed on start,
// it will happen during this test.
func cleanUp(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "Helper.NewHelper")
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
}

func getUUIDFromShill(ctx context.Context) (string, error) {
	ctx, st := timing.Start(ctx, "Helper.NewHelper")
	defer st.End()

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create cellular.Helper")
	}
	deviceProps, err := helper.Device.GetProperties(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get Device properties")
	}
	homeProvider, err := deviceProps.Get(shillconst.DevicePropertyCellularHomeProvider)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get %q property", shillconst.DevicePropertyCellularHomeProvider)
	}
	homeProviderMap, ok := homeProvider.(map[string]string)
	if !ok {
		return "", errors.New("invalid format for Home Provider property")
	}
	carrierID, ok := homeProviderMap[shillconst.OperatorUUIDKey]
	if !ok {
		return "", errors.New("home provider UUID not found")
	}
	return carrierID, nil
}
