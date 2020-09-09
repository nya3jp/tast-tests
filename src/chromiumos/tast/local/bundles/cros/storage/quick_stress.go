// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/bundles/cros/storage/stress"
	"chromiumos/tast/testing"
)

const (
	// Main storage device has to be >= 16GB.
	minDeviceSizeBytes = 16 * 1024 * 1024 * 1024
)

// testFunc is the code associated with a sub-test.
type testFunc func(context.Context, *testing.State)

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuickStress,
		Desc:         "Performs a short version of storage qualification test",
		Contacts:     []string{"chromeos-engprod-platform-syd@google.com"},
		Attr:         []string{"group:storage-qual"},
		Data:         stress.Configs,
		SoftwareDeps: []string{"storage_wearout_detect"},
		Params: []testing.Param{{
			Name:    "0_setup",
			Val:     testFunc(setup),
			Timeout: 1 * time.Hour,
		}, {
			Name:    "1_stress",
			Val:     testFunc(testBlock),
			Timeout: 4 * time.Hour,
		}, {
			Name:    "2_stress",
			Val:     testFunc(testBlock),
			Timeout: 4 * time.Hour,
		}, {
			Name:    "3_teardown",
			Val:     testFunc(teardown),
			Timeout: 1 * time.Hour,
		}},
	})
}

func setup(ctx context.Context, s *testing.State) {
	// Fetching info of all storage devices.
	info, err := stress.ReadDiskInfo(ctx)
	if err != nil {
		s.Fatal("Failed reading disk info: ", err)
	}
	s.Log(ctx, "Disk info: ", info)

	// Checking the size of the main storage device.
	err = info.CheckMainDeviceSize(minDeviceSizeBytes)
	if err != nil {
		s.Fatal("Main storage disk is too small: ", err)
	}

	// Save storage info to results.
	err = info.SaveDiskInfo(filepath.Join(s.OutDir(), "diskinfo.json"))
	if err != nil {
		s.Fatal("Error saving disk info: ", err)
	}

	// Run tests to collect metrics.
	resultWriter := &stress.FioResultWriter{}
	resultWriter.Start(ctx)
	defer resultWriter.StopAndSave(s.OutDir())

	testConfig := &stress.TestConfig{ResultWriter: resultWriter}
	stress.RunFioStressForBootDevice(ctx, s, "seq_write", testConfig)
	stress.RunFioStressForBootDevice(ctx, s, "seq_read", testConfig)
	stress.RunFioStressForBootDevice(ctx, s, "4k_write", testConfig)
	stress.RunFioStressForBootDevice(ctx, s, "4k_read", testConfig)
	stress.RunFioStressForBootDevice(ctx, s, "16k_write", testConfig)
	stress.RunFioStressForBootDevice(ctx, s, "16k_read", testConfig)
}

func testBlock(ctx context.Context, s *testing.State) {
	resultWriter := &stress.FioResultWriter{}
	resultWriter.Start(ctx)
	defer resultWriter.StopAndSave(s.OutDir())

	stress.RunFioStressForBootDevice(ctx, s, "64k_stress", &stress.TestConfig{Duration: 1 * time.Hour})
	if err := testing.Sleep(ctx, 5*time.Minute); err != nil {
		s.Fatal("Sleep failed: ", err)
	}
	stress.RunFioStressForBootDevice(ctx, s, "surfing", &stress.TestConfig{
		Duration:     1 * time.Hour,
		VerifyOnly:   true,
		ResultWriter: resultWriter,
	})
	testing.Sleep(ctx, 5*time.Minute)
	if err := testing.Sleep(ctx, 5*time.Minute); err != nil {
		s.Fatal("Sleep failed: ", err)
	}
	stress.RunFioStressForBootDevice(ctx, s, "8k_async_randwrite", &stress.TestConfig{Duration: 4 * time.Minute})
	stress.Suspend(ctx)
	stress.RunFioStressForBootDevice(ctx, s, "8k_async_randwrite", &stress.TestConfig{
		VerifyOnly:   true,
		ResultWriter: resultWriter,
	})
}

func teardown(ctx context.Context, s *testing.State) {
	// Teardown is exactly the same as setup.
	setup(ctx, s)
}

// QuickStress runs a short version of disk IO performance tests.
func QuickStress(ctx context.Context, s *testing.State) {
	s.Param().(testFunc)(ctx, s)
}
