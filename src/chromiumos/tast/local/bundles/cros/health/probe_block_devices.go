// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
)

type blockDeviceInfo struct {
	SubsystemVendor *jsontypes.Uint32 `json:"subsystem_vendor"`
	SubsystemDevice *jsontypes.Uint32 `json:"subsystem_device"`
	PcieRev         *uint8            `json:"pcie_rev"`
	FirmwareRev     *jsontypes.Uint64 `json:"firmware_rev"`
	Manfid          *uint16           `json:"manfid"`
	Pnm             *jsontypes.Uint64 `json:"pnm"`
	Prv             *uint8            `json:"prv"`
	Fwrev           *jsontypes.Uint64 `json:"fwrev"`
	JedecManfid     *uint16           `json:"jedec_manfid"`
}

type nonRemovableBlockDeviceInfo struct {
	BytesReadSinceLastBoot          jsontypes.Uint64  `json:"bytes_read_since_last_boot"`
	BytesWrittenSinceLastBoot       jsontypes.Uint64  `json:"bytes_written_since_last_boot"`
	IoTimeSecondsSinceLastBoot      jsontypes.Uint64  `json:"io_time_seconds_since_last_boot"`
	Name                            string            `json:"name"`
	Path                            string            `json:"path"`
	ReadTimeSecondsSinceLastBoot    jsontypes.Uint64  `json:"read_time_seconds_since_last_boot"`
	Serial                          jsontypes.Uint32  `json:"serial"`
	Size                            jsontypes.Uint64  `json:"size"`
	Type                            string            `json:"type"`
	WriteTimeSecondsSinceLastBoot   jsontypes.Uint64  `json:"write_time_seconds_since_last_boot"`
	DiscardTimeSecondsSinceLastBoot *jsontypes.Uint64 `json:"discard_time_seconds_since_last_boot"`
	ManufacturerID                  uint8             `json:"manufacturer_id"`
	DeviceInfo                      *blockDeviceInfo  `json:"device_info"`
}

type blockDeviceResult struct {
	BlockDevices []nonRemovableBlockDeviceInfo `json:"block_devices"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeBlockDevices,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for various probe data points",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeBlockDevices(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryStorage}
	var blockDevice blockDeviceResult
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &blockDevice); err != nil {
		s.Fatal("Failed to get storage telemetry info: ", err)
	}

	if len(blockDevice.BlockDevices) < 1 {
		s.Fatalf("Wrong number of block device: got %d; want 1+", len(blockDevice.BlockDevices))
	}
}
