// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/testing"
)

// type sortableMessage []proto.Message

func init() {
	testing.AddTest(&testing.Test{
		Func:     ModemFWManifestInstallation,
		Desc:     "Verifies that all modem FWs compatible with a device can be  installed",
		Contacts: []string{"andrewlassalle@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular"},
	})
}

// ModemFWManifestInstallation Test
func ModemFWManifestInstallation(ctx context.Context, s *testing.State) {
	modemFirmwarePath := cellular.GetModemFirmwareManifestPath()
	if _, err := os.Stat(modemFirmwarePath); err != nil {
		s.Log("Skipping test on device. Modem FW manifest not available")
		return
	}

	message, err := cellular.ParseModemFWManifest(ctx, s)
	if err != nil {
		s.Fatal("Failed to parse firmware manifest: ", err)
	}

	// Install images
	// Build a list of all the FW combinations.
	// Each carrier_firmware specifies the main_firmware_version to be used. If an oem_firmware
	// for the same device lists the same main_firmware_version, then the oem_firmware also has to flashed.
	type FWStruct struct {
		main, oem, carrier string
	}
	// firmwaresToInstall := make(map[FWStruct]bool)
	usbDevices := make(map[string]bool)
	for _, device := range message.Device {
		if !strings.HasPrefix(device.DeviceId, "usb:") {
			s.Fatal("Device type not supported by Tast test:", device.DeviceId)
		}
		usbID := strings.TrimLeft(device.DeviceId, "usb:")
		if _, ok := usbDevices[usbID]; !ok {
			usbDevices[usbID] = checkIfUsbDeviceExists(ctx, s, usbID)
		}
		if !usbDevices[usbID] {
			s.Log("Skipping test on particular device since it doesn't have a USB device with ID: ", usbID)
			continue
		}
		// for _, carrierFW := range device.CarrierFirmware {
		// 	if !checkIfFirmwareExists(filepath.Join(modemFirmwarePath, carrierFW.Filename)) {
		// 		missingFiles = append(missingFiles, carrierFW.Filename)
		// 	}
		// 	// Verify if main FWs used by carrier FW exist.
		// 	if carrierFW.MainFirmwareVersion != "" && !mainFirmwares[carrierFW.MainFirmwareVersion] {
		// 		s.Fatal("Main firmware '", carrierFW.MainFirmwareVersion, "' referenced by carrier FW '", carrierFW.Version, "' does not exist.")
		// 	}
		// }
	}
}

func installMainFirmware(path string) bool {
	return false
}

// checkIfUsbDeviceExists Check if a USB device with ID in the form of XXXX:YYYY exists on the device.
func checkIfUsbDeviceExists(ctx context.Context, s *testing.State, ID string) bool {
	b, err := testexec.CommandContext(ctx, "lsusb", "-d", ID).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run lsusb")
		return false
	}
	output := string(b)
	return strings.Contains(output, ID)
}
