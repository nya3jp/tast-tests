// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/proto"

	mfv2 "chromiumos/modemfwd"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ParseModemFWManifest the modem firmware manifest.
func ParseModemFWManifest(ctx context.Context, s *testing.State) (*mfv2.FirmwareManifestV2, error) {
	modemFirmwareProtoPath := GetModemFirmwareManifestPath()
	if _, err := os.Stat(modemFirmwareProtoPath); err != nil {
		return nil, errors.Wrap(err, "modem FW manifest not available")
	}

	output, err := testexec.CommandContext(ctx, "cat", modemFirmwareProtoPath).Output()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to access the firmware manifest: %s", modemFirmwareProtoPath)
	}

	s.Log("Parsing modem firmware proto")
	message := &mfv2.FirmwareManifestV2{}
	if err := proto.UnmarshalText(string(output), message); err != nil {
		return nil, errors.Wrapf(err, "failed to parse firmware manifest: %s", modemFirmwareProtoPath)
	}
	s.Log("Parsed successfully")

	return message, nil
}

// GetModemFirmwarePath Get the path where the modem firmware files are located.
func GetModemFirmwarePath() string {
	return "/opt/google/modemfwd-firmware/"

}

// GetModemFirmwareManifestPath Get the path of the modem firmware manifest.
func GetModemFirmwareManifestPath() string {
	return filepath.Join(GetModemFirmwarePath(), "firmware_manifest.prototxt")
}
