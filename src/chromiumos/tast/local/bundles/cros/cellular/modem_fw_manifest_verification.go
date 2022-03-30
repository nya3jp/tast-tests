// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/dlc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModemFWManifestVerification,
		Desc:         "Verifies the validity of the firmware manifest",
		Contacts:     []string{"andrewlassalle@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_sim_active", "cellular_cq", "cellular_ota_avl"},
		SoftwareDeps: []string{"modemfwd"},
	})
}

// ModemFWManifestVerification Test
func ModemFWManifestVerification(ctx context.Context, s *testing.State) {
	fileExists := func(file string) bool {
		_, err := os.Stat(file)
		return !os.IsNotExist(err)
	}

	if !fileExists(cellular.GetModemFirmwareManifestPath()) {
		s.Fatal("Cannot find ", cellular.GetModemFirmwareManifestPath())
	}

	modemFirmwarePath := cellular.GetModemFirmwarePath()
	manifest, err := cellular.ParseModemFirmwareManifest(ctx)
	if err != nil {
		s.Fatal("Failed to parse the firmware manifest: ", err)
	}

	missingFiles := make(map[string]bool)
	var mainFirmwares map[string]bool
	for _, device := range manifest.Device {
		// Verify that we don't have repeated main FWs.
		mainFirmwares = make(map[string]bool)
		for _, mainFirmware := range device.MainFirmware {
			if mainFirmwares[mainFirmware.Version] {
				s.Fatalf("Repeated value for main firmware: %q in variant: %q", mainFirmware.Version, device.Variant)
			}
			mainFirmwares[mainFirmware.Version] = true
		}
		// Verify that all files exist.
		for _, mainFW := range device.MainFirmware {
			if !fileExists(filepath.Join(modemFirmwarePath, mainFW.Filename)) {
				missingFiles[mainFW.Filename] = true
			}
		}
		for _, oemFW := range device.OemFirmware {
			if !fileExists(filepath.Join(modemFirmwarePath, oemFW.Filename)) {
				missingFiles[oemFW.Filename] = true
			}
			// Verify if main FWs used by OEM FW exist.
			for _, oemVersion := range oemFW.MainFirmwareVersion {
				if !mainFirmwares[oemVersion] {
					s.Fatalf("Main firmware %q referenced by OEM FW %q does not exist", oemVersion, oemFW.Version)
				}
			}
		}
		for _, carrierFW := range device.CarrierFirmware {
			if !fileExists(filepath.Join(modemFirmwarePath, carrierFW.Filename)) {
				missingFiles[carrierFW.Filename] = true
			}
			// Verify if main FWs used by carrier FW exist.
			if carrierFW.MainFirmwareVersion != "" && !mainFirmwares[carrierFW.MainFirmwareVersion] {
				s.Fatalf("Main firmware %q referenced by carrier FW %q does not exist", carrierFW.MainFirmwareVersion, carrierFW.Version)
			}
			if len(carrierFW.CarrierId) == 0 {
				s.Fatalf("There is no carrier id defined for carrier FW %q", carrierFW.Version)
			}
		}
	}

	// Verify that the DLC exists in the dlcservice manifest
	for _, device := range manifest.Device {
		if device.GetDlcId() != "" {
			_, err := dlc.GetDlcState(ctx, device.DlcId)

			if err != nil {
				s.Fatalf("Failed to get state info for DLC %q: %q", device.DlcId, err)
			}
		}
	}

	if len(missingFiles) > 0 {
		keys := make([]string, 0, len(missingFiles))
		for k := range missingFiles {
			keys = append(keys, k)
		}
		s.Fatalf("The following firmware files are specified in the manifest, but don't exist: %q", keys)
	}
}
