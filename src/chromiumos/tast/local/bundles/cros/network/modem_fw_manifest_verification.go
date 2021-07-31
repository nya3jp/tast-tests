// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ModemFWManifestVerification,
		Desc:     "Verifies the validity of the firmware manifest",
		Contacts: []string{"andrewlassalle@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular"},
	})
}

// ModemFWManifestVerification Test
func ModemFWManifestVerification(ctx context.Context, s *testing.State) {
	modemFirmwarePath := cellular.GetModemFirmwareManifestPath()
	if _, err := os.Stat(modemFirmwarePath); err != nil {
		s.Log("Skipping test on device. Modem FW manifest not available")
		return
	}

	message, err := cellular.ParseModemFWManifest(ctx, s)
	if err != nil {
		s.Fatal("Failed to parse the firmware manifest: ", err)
	}

	var missingFiles []string
	var mainFirmwares map[string]bool
	for _, device := range message.Device {
		// Verify that we don't have repeated main FWs.
		mainFirmwares = make(map[string]bool)
		for _, mainFirmware := range device.MainFirmware {
			if mainFirmwares[mainFirmware.Version] {
				s.Fatal("Repeated value for main firmware: ", mainFirmware.Version, " in variant:", device.Variant)
			} else {
				mainFirmwares[mainFirmware.Version] = true
			}
		}
		// Verify that all files exist.
		for _, mainFW := range device.MainFirmware {
			if !checkIfFirmwareExists(filepath.Join(modemFirmwarePath, mainFW.Filename)) {
				missingFiles = append(missingFiles, mainFW.Filename)
			}
		}
		for _, oemFW := range device.OemFirmware {
			if !checkIfFirmwareExists(filepath.Join(modemFirmwarePath, oemFW.Filename)) {
				missingFiles = append(missingFiles, oemFW.Filename)
			}
			// Verify if main FWs used by OEM FW exist.
			for _, oemMainFirmwareVersion := range oemFW.MainFirmwareVersion {
				if !mainFirmwares[oemMainFirmwareVersion] {
					s.Fatal("Main firmware '", oemMainFirmwareVersion, "' referenced by OEM FW '", oemFW.Version, "' does not exist.")
				}
			}
		}
		for _, carrierFW := range device.CarrierFirmware {
			if !checkIfFirmwareExists(filepath.Join(modemFirmwarePath, carrierFW.Filename)) {
				missingFiles = append(missingFiles, carrierFW.Filename)
			}
			// Verify if main FWs used by carrier FW exist.
			if carrierFW.MainFirmwareVersion != "" && !mainFirmwares[carrierFW.MainFirmwareVersion] {
				s.Fatal("Main firmware '", carrierFW.MainFirmwareVersion, "' referenced by carrier FW '", carrierFW.Version, "' does not exist.")
			}
		}
	}

	if len(missingFiles) > 0 {
		s.Fatal("The following firmwares files are specified in the manifest, but don't exist: '", strings.Join(missingFiles, ","))
	}
}

// checkIfFirmwareExists Returns true if directory exists, false otherwise
func checkIfFirmwareExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}
