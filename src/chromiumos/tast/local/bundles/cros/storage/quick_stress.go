// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"path/filepath"
	"strconv"
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

func isDualQual(ctx context.Context, s *testing.State) bool {
	if val, ok := s.Var("storage.QuickStress.slcQual"); ok {
		dual, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Cannot parse argumet 'storage.QuickStress.slcQual' of type bool: ", err)
		}
		return dual
	}
	return false
}

func getSlcDevice(ctx context.Context, s *testing.State) string {
	info, err := stress.ReadDiskInfo(ctx)
	if err != nil {
		s.Fatal("Failed reading disk info: ", err)
	}
	slc, err := info.SlcDevice()
	if slc == nil {
		s.Fatal("Dual qual is specified but SLC device is not present: ", err)
	}
	return "/dev/" + slc.Name
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuickStress,
		Desc:         "Performs a short version of storage qualification test",
		Contacts:     []string{"chromeos-engprod-platform-syd@google.com"},
		Attr:         []string{"group:storage-qual"},
		Data:         stress.Configs,
		SoftwareDeps: []string{"storage_wearout_detect"},
		Vars:         []string{"storage.QuickStress.slcQual"},
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
	check(ctx, s)

	// Run tests to collect metrics.
	resultWriter := &stress.FioResultWriter{}
	defer resultWriter.Save(ctx, s.OutDir())

	resultWriter.RunSequential(ctx, s, []stress.FioBlock{{"main", benchmarkMain}, {"slc", benchmarkSlc}})
}

func testBlock(ctx context.Context, s *testing.State) {
	resultWriter := &stress.FioResultWriter{}
	defer resultWriter.Save(ctx, s.OutDir())

	resultWriter.RunParallel(ctx, s, []stress.FioBlock{{"stress_main", testBlockMain}, {"stress_slc", testBlockSlc}})
	stress.Suspend(ctx)

	resultWriter.RunSequential(ctx, s, []stress.FioBlock{{"verify_main", verifyBlockMain}, {"verify_slc", verifyBlockSlc}})
}

func teardown(ctx context.Context, s *testing.State) {
	// Run tests to collect metrics.
	resultWriter := &stress.FioResultWriter{}
	defer resultWriter.Save(ctx, s.OutDir())

	resultWriter.RunSequential(ctx, s, []stress.FioBlock{{"after_main", benchmarkMain}, {"after_slc", benchmarkSlc}})
}

func check(ctx context.Context, s *testing.State) {
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

	if isDualQual(ctx, s) {
		slc, err := info.SlcDevice()
		if slc == nil {
			s.Fatal("Dual qual is specified but SLC device is not present: ", err)
		}
	}
}

func benchmarkMain(ctx context.Context, s *testing.State, resultWriter *stress.FioResultWriter) {
	testConfig := &stress.TestConfig{ResultWriter: resultWriter, Path: stress.BootDeviceFioPath}
	stress.RunFioStress(ctx, s, testConfig.WithJob("seq_write"))
	stress.RunFioStress(ctx, s, testConfig.WithJob("seq_read"))
	stress.RunFioStress(ctx, s, testConfig.WithJob("4k_write"))
	stress.RunFioStress(ctx, s, testConfig.WithJob("4k_read"))
	stress.RunFioStress(ctx, s, testConfig.WithJob("16k_write"))
	stress.RunFioStress(ctx, s, testConfig.WithJob("16k_read"))
}

func benchmarkSlc(ctx context.Context, s *testing.State, resultWriter *stress.FioResultWriter) {
	if !isDualQual(ctx, s) {
		return
	}
	testConfig := &stress.TestConfig{ResultWriter: resultWriter, Path: getSlcDevice(ctx, s)}
	stress.RunFioStress(ctx, s, testConfig.WithJob("4k_write"))
	stress.RunFioStress(ctx, s, testConfig.WithJob("4k_read"))
}

func testBlockMain(ctx context.Context, s *testing.State, resultWriter *stress.FioResultWriter) {
	testConfig := &stress.TestConfig{Path: stress.BootDeviceFioPath}

	stress.RunFioStress(ctx, s,
		testConfig.
			WithJob("64k_stress").
			WithDuration(1*time.Hour))
	if err := testing.Sleep(ctx, 5*time.Minute); err != nil {
		s.Fatal("Sleep failed: ", err)
	}
	stress.RunFioStress(ctx, s,
		testConfig.
			WithJob("surfing").
			WithDuration(1*time.Hour).
			WithVerifyOnly(true).
			WithResultWriter(resultWriter))

	if err := testing.Sleep(ctx, 5*time.Minute); err != nil {
		s.Fatal("Sleep failed: ", err)
	}

	stress.RunFioStress(ctx, s,
		testConfig.
			WithJob("8k_async_randwrite").
			WithDuration(4*time.Minute))
}

func testBlockSlc(ctx context.Context, s *testing.State, resultWriter *stress.FioResultWriter) {
	if !isDualQual(ctx, s) {
		return
	}
	testConfig := &stress.TestConfig{Path: getSlcDevice(ctx, s)}

	stress.RunFioStress(ctx, s,
		testConfig.
			WithJob("4k_write").
			WithDuration(1*time.Hour))
	if err := testing.Sleep(ctx, 5*time.Minute); err != nil {
		s.Fatal("Sleep failed: ", err)
	}
	stress.RunFioStress(ctx, s,
		testConfig.
			WithJob("4k_write").
			WithDuration(1*time.Hour).
			WithVerifyOnly(true).
			WithResultWriter(resultWriter))
	if err := testing.Sleep(ctx, 5*time.Minute); err != nil {
		s.Fatal("Sleep failed: ", err)
	}
	stress.RunFioStress(ctx, s,
		testConfig.
			WithJob("4k_write").
			WithDuration(4*time.Minute))
}

func verifyBlockMain(ctx context.Context, s *testing.State, resultWriter *stress.FioResultWriter) {
	testConfig := &stress.TestConfig{Path: stress.BootDeviceFioPath}
	stress.RunFioStress(ctx, s,
		testConfig.
			WithJob("8k_async_randwrite").
			WithVerifyOnly(true).
			WithResultWriter(resultWriter))
}

func verifyBlockSlc(ctx context.Context, s *testing.State, resultWriter *stress.FioResultWriter) {
	if !isDualQual(ctx, s) {
		return
	}
	testConfig := &stress.TestConfig{Path: getSlcDevice(ctx, s)}
	stress.RunFioStress(ctx, s,
		testConfig.
			WithJob("4k_write").
			WithVerifyOnly(true).
			WithResultWriter(resultWriter))
}

// QuickStress runs a short version of disk IO performance tests.
func QuickStress(ctx context.Context, s *testing.State) {
	s.Param().(testFunc)(ctx, s)
}
