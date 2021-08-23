// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"path/filepath"

	"github.com/golang/protobuf/proto"
	// The contents of chromiumos/modemfwd are built and generated in platform2/modemfwd/.
	mfwd "chromiumos/modemfwd"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ParseModemFirmwareManifest Parses the modem firmware manifest and returns the FirmwareManifestV2 proto object.
func ParseModemFirmwareManifest(ctx context.Context, s *testing.State) (*mfwd.FirmwareManifestV2, error) {
	modemFirmwareProtoPath := GetModemFirmwareManifestPath()
	output, err := testexec.CommandContext(ctx, "cat", modemFirmwareProtoPath).Output()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to access the firmware manifest: %s", modemFirmwareProtoPath)
	}

	s.Log("Parsing modem firmware proto")
	manifest := &mfwd.FirmwareManifestV2{}
	if err := proto.UnmarshalText(string(output), manifest); err != nil {
		return nil, errors.Wrapf(err, "failed to parse firmware manifest: %s", modemFirmwareProtoPath)
	}
	s.Log("Parsed successfully")

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
func ParseModemHelperManifest(ctx context.Context, s *testing.State) (*mfwd.HelperManifest, error) {
	modemHelperProtoPath := GetModemHelperManifestPath()
	output, err := testexec.CommandContext(ctx, "cat", modemHelperProtoPath).Output()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to access the helper manifest: %s", modemHelperProtoPath)
	}

	s.Log("Parsing modem helper proto")
	manifest := &mfwd.HelperManifest{}
	if err := proto.UnmarshalText(string(output), manifest); err != nil {
		return nil, errors.Wrapf(err, "failed to parse helper manifest: %s", modemHelperProtoPath)
	}
	s.Log("Parsed successfully")

	return manifest, nil
}

// GetModemHelperPath Get the path where the modem helper files are located.
func GetModemHelperPath() string {
	return "/opt/google/modemfwd-helpers/"
}

// GetModemHelperManifestPath Get the path of the modem helper manifest.
func GetModemHelperManifestPath() string {
	return filepath.Join(GetModemHelperPath(), "helper_manifest.prototxt")
}
