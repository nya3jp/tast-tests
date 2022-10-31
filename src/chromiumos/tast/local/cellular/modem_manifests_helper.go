// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cellular provides functions for testing Cellular connectivity.
package cellular

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	// The contents of chromiumos/modemfwd are built and generated in platform2/modemfwd/.
	mfwd "chromiumos/modemfwd"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/modemfwd"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// ParseModemFirmwareManifest Parses the modem firmware manifest and returns the FirmwareManifestV2 proto object.
func ParseModemFirmwareManifest(ctx context.Context) (*mfwd.FirmwareManifestV2, error) {
	ctx, st := timing.Start(ctx, "ParseModemFirmwareManifest")
	defer st.End()

	modemFirmwareProtoPath := GetModemFirmwareManifestPath()
	output, err := testexec.CommandContext(ctx, "cat", modemFirmwareProtoPath).Output()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to access the firmware manifest: %s", modemFirmwareProtoPath)
	}

	testing.ContextLog(ctx, "Parsing modem firmware proto")
	manifest := &mfwd.FirmwareManifestV2{}
	if err := proto.UnmarshalText(string(output), manifest); err != nil {
		return nil, errors.Wrapf(err, "failed to parse firmware manifest: %s", modemFirmwareProtoPath)
	}
	testing.ContextLog(ctx, "Parsed successfully")

	return manifest, nil
}

// GetModemFirmwarePath Get the path where the modem firmware files are located.
func GetModemFirmwarePath() string {
	return "/opt/google/modemfwd-firmware/"
}

// GetModemFirmwareManifestPath Get the path of the modem firmware manifest.
func GetModemFirmwareManifestPath() string {
	return filepath.Join(GetModemFirmwarePath(), "firmware_manifest.prototxt")
}

// ParseModemHelperManifest Parses the modem helper manifest and returns the HelperManifest proto object.
func ParseModemHelperManifest(ctx context.Context) (*mfwd.HelperManifest, error) {
	ctx, st := timing.Start(ctx, "ParseModemHelperManifest")
	defer st.End()

	modemHelperProtoPath := GetModemHelperManifestPath()
	output, err := testexec.CommandContext(ctx, "cat", modemHelperProtoPath).Output()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to access the helper manifest: %s", modemHelperProtoPath)
	}

	testing.ContextLog(ctx, "Parsing modem helper proto")
	manifest := &mfwd.HelperManifest{}
	if err := proto.UnmarshalText(string(output), manifest); err != nil {
		return nil, errors.Wrapf(err, "failed to parse helper manifest: %s", modemHelperProtoPath)
	}
	testing.ContextLog(ctx, "Parsed successfully")

	return manifest, nil
}

// GetModemFirmwareDevice gets the modem firmware device for this variant.
func GetModemFirmwareDevice(ctx context.Context) (*mfwd.Device, error) {
	// Ignore error since some boards may not always use a variant
	dutVariant, _ := GetDeviceVariant(ctx)

	manifest, err := ParseModemFirmwareManifest(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse the firmware manifest")
	}

	for _, device := range manifest.Device {
		if dutVariant == device.Variant || dutVariant == "" {
			return device, nil
		}
	}

	return nil, errors.Errorf("variant %q does not contain a device", dutVariant)
}

// GetModemHelperPath Get the path where the modem helper files are located.
func GetModemHelperPath() string {
	return "/opt/google/modemfwd-helpers/"
}

// GetModemHelperManifestPath Get the path of the modem helper manifest.
func GetModemHelperManifestPath() string {
	return filepath.Join(GetModemHelperPath(), "helper_manifest.prototxt")
}

// ModemHelperPathExists returns true if the modem manifest helper path exists on this device.
func ModemHelperPathExists() bool {
	_, err := os.Stat(GetModemHelperPath())
	return err == nil
}

// GetModemFirmwareHelperEntry gets the modem helper entry for this variant.
func GetModemFirmwareHelperEntry(ctx context.Context) (*mfwd.HelperEntry, error) {
	// Ignore error since some boards may not always use a variant
	dutVariant, _ := GetDeviceVariant(ctx)

	helperManifest, err := ParseModemHelperManifest(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get extra arguments from firmware proto")
	}

	for _, helper := range helperManifest.Helper {
		if len(helper.Variant) == 0 || dutVariant == "" {
			return helper, nil
		}

		for _, variant := range helper.Variant {
			if variant == dutVariant {
				return helper, nil
			}
		}
	}

	return nil, errors.Errorf("variant %q does not contain a helper entry", dutVariant)
}

// GetDlcIDForVariant gets the dlc id of the variant, otherwise return error.
// By default, the go proto helper will return an empty string if there is no DlcId value.
func GetDlcIDForVariant(ctx context.Context) (string, error) {
	dutVariant, err := GetDeviceVariant(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get device variant")
	}

	manifest, err := ParseModemFirmwareManifest(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse the firmware manifest")
	}
	for _, device := range manifest.Device {
		if dutVariant == device.Variant {
			if device.GetDlc() != nil {
				return device.GetDlc().GetDlcId(), nil
			}
		}
	}
	return "", errors.Errorf("variant %q does not contain a DlcId", dutVariant)

}

// RestartModemWithHelper uses the modemfwd helper to force a modem restart.
func RestartModemWithHelper(ctx context.Context) error {
	helper, err := GetModemFirmwareHelperEntry(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get modem firmware helper")
	}

	device, err := GetModemFirmwareDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get modem device")
	}

	// Verify USB device exists before attempting to restart
	const usbPrefix = "usb:"
	if err := modemfwd.WaitForUsbDevice(ctx, strings.TrimPrefix(device.DeviceId, usbPrefix), time.Minute); err != nil {
		return errors.Wrap(err, "failed to find modem USB device")
	}

	helperPath := filepath.Join(GetModemHelperPath(), helper.Filename)
	args := helper.ExtraArgument
	args = append([]string{"--reboot"}, args...)
	if err := testexec.CommandContext(ctx, helperPath, args...).Run(); err != nil {
		return errors.Wrap(err, "failed to restart modem with modemfwd-helper")
	}

	// Wait for MM to export the modem after rebooting
	if _, err = modemmanager.NewModem(ctx); err != nil {
		return errors.Wrap(err, "failed to get modem after reboot")
	}
	return nil
}
