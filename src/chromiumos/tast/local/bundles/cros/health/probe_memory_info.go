// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"encoding/json"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
)

type memoryInfo struct {
	AvailableMemoryKib      jsontypes.Uint32 `json:"available_memory_kib"`
	FreeMemoryKib           jsontypes.Uint32 `json:"free_memory_kib"`
	PageFaultsSinceLastBoot jsontypes.Uint64 `json:"page_faults_since_last_boot"`
	TotalMemoryKib          jsontypes.Uint32 `json:"total_memory_kib"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeMemoryInfo,
		Desc: "Check that we can probe cros_healthd for memory info",
		Contacts: []string{
			"pmoy@google.com",
			"cros-tdm@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func validateMemoryData(memory memoryInfo) error {
	// Each memory metric should be a positive integer. This assumes that all
	// machines will always have at least 1 free KiB of memory, and all machines
	// will have page faulted at least once between boot and the time this test
	// finishes executing.
	if memory.AvailableMemoryKib <= 0 {
		return errors.Errorf("Failed. available_memory_kib is not greater than zero: %d", memory.AvailableMemoryKib)
	}

	if memory.FreeMemoryKib <= 0 {
		return errors.Errorf("Failed. free_memory_kib is not greater than zero: %d", memory.FreeMemoryKib)
	}

	if memory.TotalMemoryKib <= 0 {
		return errors.Errorf("Failed. total_memory_kib is not greater than zero: %d", memory.TotalMemoryKib)
	}

	if memory.PageFaultsSinceLastBoot <= 0 {
		return errors.Errorf("Failed. page_faults_since_last_boot is not greater than zero: %d", memory.PageFaultsSinceLastBoot)
	}

	return nil
}

func ProbeMemoryInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryMemory}
	rawData, err := croshealthd.RunTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get memory telemetry info: ", err)
	}

	dec := json.NewDecoder(strings.NewReader(string(rawData)))
	dec.DisallowUnknownFields()

	var memory memoryInfo
	if err := dec.Decode(&memory); err != nil {
		s.Fatalf("Failed to decode memory data [%q], err [%v]", rawData, err)
	}

	if err := validateMemoryData(memory); err != nil {
		s.Fatalf("Failed to validate memory data, err [%v]", err)
	}
}
