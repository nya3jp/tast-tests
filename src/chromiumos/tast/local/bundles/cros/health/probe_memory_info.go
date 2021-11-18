// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type memoryEncryptionInfo struct {
	ActiveAlgrithm  string           `json:"active_algorithm"`
	EncryptionState string           `json:"encryption_state"`
	KeyLength       jsontypes.Uint32 `json:"key_length"`
	MaxKeyNumber    jsontypes.Uint32 `json:"max_key_number"`
}

type memoryInfo struct {
	AvailableMemoryKib      jsontypes.Uint32     `json:"available_memory_kib"`
	FreeMemoryKib           jsontypes.Uint32     `json:"free_memory_kib"`
	PageFaultsSinceLastBoot jsontypes.Uint64     `json:"page_faults_since_last_boot"`
	TotalMemoryKib          jsontypes.Uint32     `json:"total_memory_kib"`
	MemoryEncryptionInfo    memoryEncryptionInfo `json:"memory_encryption_info"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeMemoryInfo,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Check that we can probe cros_healthd for memory info",
		Contacts: []string{
			"pmoy@google.com",
			"cros-tdm@google.com",
			"pathan.jilani@intel.com",
			"intel-chrome-system-automation-team@intel.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
		Params: []testing.Param{{
			Val: false,
		}, {
			Name: "memory_encryption",
			Val:  true,
			// TODO(b/207569436): Define hardware dependency and get rid of hard-coding the models.
			ExtraHardwareDeps: hwdep.D(hwdep.Model("brya", "redrix", "kano", "anahera", "primus", "crota")),
			ExtraAttr:         []string{"informational"},
		}},
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

func validateMemoryEncryptionData(memoryEncryption memoryEncryptionInfo) error {
	if memoryEncryption.EncryptionState == "" {
		return errors.New("failed to verify EncryptionState")
	}
	if memoryEncryption.KeyLength <= 0 {
		return errors.Errorf("failed to verify KeyLength : %d", memoryEncryption.KeyLength)
	}
	if memoryEncryption.MaxKeyNumber <= 0 {
		return errors.Errorf("failed to verify MaxKeyNumber: %d", memoryEncryption.MaxKeyNumber)
	}
	if memoryEncryption.ActiveAlgrithm == "" {
		return errors.New("failed to verify  ActiveAlgrithm")
	}
	return nil
}

func ProbeMemoryInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryMemory}
	var memory memoryInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &memory); err != nil {
		s.Fatal("Failed to get memory telemetry info: ", err)
	}

	if err := validateMemoryData(memory); err != nil {
		s.Fatalf("Failed to validate memory data, err [%v]", err)
	}

	if s.Param().(bool) {
		if err := validateMemoryEncryptionData(memory.MemoryEncryptionInfo); err != nil {
			s.Fatal("Failed to validate Memory Encryption data: ", err)
		}
	}
}
