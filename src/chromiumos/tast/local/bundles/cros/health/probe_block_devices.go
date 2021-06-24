// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"encoding/json"
	"strings"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

type blockDeviceInfo struct {
	BytesReadSinceLastBoot          string  `json:"bytes_read_since_last_boot"`
	BytesWrittenSinceLastBoot       string  `json:"bytes_written_since_last_boot"`
	IoTimeSecondsSinceLastBoot      string  `json:"io_time_seconds_since_last_boot"`
	Name                            string  `json:"name"`
	Path                            string  `json:"path"`
	ReadTimeSecondsSinceLastBoot    string  `json:"read_time_seconds_since_last_boot"`
	Serial                          string  `json:"serial"`
	Size                            string  `json:"size"`
	Type                            string  `json:"type"`
	WriteTimeSecondsSinceLastBoot   string  `json:"write_time_seconds_since_last_boot"`
	DiscardTimeSecondsSinceLastBoot *string `json:"discard_time_seconds_since_last_boot"`
	ManufacturerID                  int     `json:"manufacturer_id"`
}

type blockDeviceResult struct {
	BlockDevices []blockDeviceInfo `json:"block_devices"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeBlockDevices,
		Desc: "Check that we can probe cros_healthd for various probe data points",
		Contacts: []string{
			"khegde@google.com",
			"pmoy@google.com",
			"cros-tdm@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeBlockDevices(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryStorage}
	rawData, err := croshealthd.RunTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get storage telemetry info: ", err)
	}

	dec := json.NewDecoder(strings.NewReader(string(rawData)))
	dec.DisallowUnknownFields()

	var blockDevice blockDeviceResult
	if err := dec.Decode(&blockDevice); err != nil {
		s.Fatalf("Failed to decode storage data [%q], err [%v]", rawData, err)
	}

	if len(blockDevice.BlockDevices) < 1 {
		s.Fatalf("Wrong number of block device: got %d; want 1+", len(blockDevice.BlockDevices))
	}
}
