// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/dlc"
	"chromiumos/tast/local/modemfwd"
	"chromiumos/tast/local/upstart"
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

	// Stop modemfwd so that the DLCs don't get uninstalled in the middle of the test.
	if err := upstart.StopJob(ctx, modemfwd.JobName); err != nil {
		s.Fatalf("Failed to stop %q: %s", modemfwd.JobName, err)
	}

	if !fileExists(cellular.GetModemFirmwareManifestPath()) {
		s.Fatal("Cannot find ", cellular.GetModemFirmwareManifestPath())
	}

	manifest, err := cellular.ParseModemFirmwareManifest(ctx)
	if err != nil {
		s.Fatal("Failed to parse the firmware manifest: ", err)
	}

	// Process the error only if the board uses variants. Older boards didn't
	// always use variants since some of them only had one of a kind.
	dutVariant, dutVariantErr := cellular.GetDeviceVariant(ctx)

	missingFiles := make(map[string]bool)
	var mainFirmwares map[string]bool
	dlcCounter := 0
	for _, device := range manifest.Device {
		// rootfs path
		modemFirmwarePaths := []string{cellular.GetModemFirmwarePath()}

		if device.GetDlcId() != "" {
			if dutVariantErr != nil {
				s.Fatalf("Failed to get device variant: %s", dutVariantErr)
			}
			dlcCounter++
			// Only the variant that matches the device's variant will contain a DLC that is
			// already installed by modemfwd.
			// Manually install the DLCs for other variants. This won't work on non test
			// images since the DLCs are purged.  It works on test images because the modem
			// DLCs have the DLC_PRELOAD flag in their ebuilds.
			if dutVariant != device.Variant {
				dlc.Install(ctx, device.DlcId, "")
			}
			state, err := dlc.GetDlcState(ctx, device.DlcId)
			// Verify that the DLC exists in the dlcservice manifest
			if err != nil {
				s.Fatalf("Failed to get state info for DLC %q: %q", device.DlcId, err)
			}
			if state.RootPath == "" {
				s.Fatalf("Failed to get mount path for DLC %q", device.DlcId)
			}
			// Append the DLC mount path for verification.
			// Until DLC is fully proven to work 100% for modemfwd, we keep a copy of all
			// modem FWs in the rootfs, so we need to verify that the files are in both locations.
			// When it's time to remove those images from the roofs, only one path will be needed,
			// and the DLC path will override the rootfs path.
			modemFirmwarePaths = append(modemFirmwarePaths, state.RootPath)
		}

		for _, modemFirmwarePath := range modemFirmwarePaths {
			s.Logf("Firmware path location for variant %q: %q", device.Variant, modemFirmwarePath)
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
				fullPath := filepath.Join(modemFirmwarePath, mainFW.Filename)
				if !fileExists(fullPath) {
					missingFiles[fullPath] = true
				}
				for _, associatedFW := range mainFW.AssocFirmware {
					fullPath := filepath.Join(modemFirmwarePath, associatedFW.Filename)
					if !fileExists(fullPath) {
						missingFiles[fullPath] = true
					}
				}
			}
			for _, oemFW := range device.OemFirmware {
				fullPath := filepath.Join(modemFirmwarePath, oemFW.Filename)
				if !fileExists(fullPath) {
					missingFiles[fullPath] = true
				}
				// Verify if main FWs used by OEM FW exist.
				for _, oemVersion := range oemFW.MainFirmwareVersion {
					if !mainFirmwares[oemVersion] {
						s.Fatalf("Main firmware %q referenced by OEM FW %q does not exist", oemVersion, oemFW.Version)
					}
				}
			}
			for _, carrierFW := range device.CarrierFirmware {
				fullPath := filepath.Join(modemFirmwarePath, carrierFW.Filename)
				if !fileExists(fullPath) {
					missingFiles[fullPath] = true
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
	}

	if len(missingFiles) > 0 {
		keys := make([]string, 0, len(missingFiles))
		for k := range missingFiles {
			keys = append(keys, k)
		}
		s.Fatalf("The following firmware files are specified in the manifest, but don't exist: %q", keys)
	}

	if dlcCounter > 0 && dlcCounter != len(manifest.Device) {
		err := cellular.TagKnownBugOnBoard(ctx, nil, "b/250065904", []string{"herobrine"})
		s.Fatal("There is an unequal number of variants and DLCs: ", err)
	}
}
