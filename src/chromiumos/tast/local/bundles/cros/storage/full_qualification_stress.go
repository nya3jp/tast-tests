// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Prior contents of this file copied to util/test_blocks.go, util/test_runner.go and
// util/test_setup.go

package storage

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/local/bundles/cros/storage/util"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FullQualificationStress,
		Desc:         "Performs a full version of storage qualification test",
		Contacts:     []string{"chromeos-engprod-platform-syd@google.com"},
		Attr:         []string{"group:storage-qual"},
		Data:         util.Configs,
		SoftwareDeps: []string{"storage_wearout_detect"},
		Vars:         []string{"tast_disk_size_gb", "tast_storage_slc_qual", "tast_suspend_block_timeout", "tast_skip_setup_check", "tast_skip_s0ix_check"},
		Params: []testing.Param{{
			Name:    "setup_benchmarks",
			Val:     util.SetupBenchmarks,
			Timeout: 1 * time.Hour,
		}, {
			Name:    "stress",
			Val:     util.StressRunner,
			Timeout: 8 * time.Hour,
		}, {
			Name:    "functional",
			Val:     util.FunctionalRunner,
			Timeout: 2 * time.Hour,
		}, {
			Name:    "mini_soak",
			Val:     util.MiniSoakRunner,
			Timeout: 2 * time.Hour,
		}, {
			Name:    "teardown_benchmarks",
			Val:     util.SetupBenchmarks,
			Timeout: 1 * time.Hour,
		}},
	})
}

// FullQualificationStress runs a full version of disk IO qualification test.
// The full run of the test can take anything between 2-14 days.
func FullQualificationStress(ctx context.Context, s *testing.State) {
	subtest := s.Param().(func(context.Context, *testing.State, *util.FioResultWriter, util.QualParam))
	start := time.Now()

	testParam := util.QualParam{}
	if val, ok := s.Var("tast_storage_slc_qual"); ok {
		var err error
		if testParam.IsSlcEnabled, err = strconv.ParseBool(val); err != nil {
			s.Fatal("Cannot parse argumet 'storage.QuickUtil.slcQual' of type bool: ", err)
		}
		if testParam.IsSlcEnabled {
			// Run tests to collect metrics for Slc device.
			if testParam.SlcDevice, err = util.SlcDevice(ctx); err != nil {
				s.Fatal("Failed to get slc device: ", err)
			}
			if err = util.Swapoff(ctx); err != nil {
				s.Fatal("Failed to turn off the swap: ", err)
			}
		}
	}

	testParam.RetentionBlockTimeout = util.DefaultRetentionBlockTimeout
	testParam.SuspendBlockTimeout = util.DefaultSuspendBlockTimeout
	testParam.SkipS0iXResidencyCheck = false
	testParam.TestDevice = util.BootDeviceFioPath

	if val, ok := s.Var("tast_suspend_block_timeout"); ok {
		var err error
		if testParam.SuspendBlockTimeout, err = time.ParseDuration(val); err != nil {
			s.Fatal("Cannot parse argument 'tast_suspend_block_timeout' of type Duration: ", err)
		}
	}

	if val, ok := s.Var("tast_skip_s0ix_check"); ok {
		var err error
		if testParam.SkipS0iXResidencyCheck, err = strconv.ParseBool(val); err != nil {
			s.Fatal("Cannot parse argument 'tast_skip_s0ix_check' of type bool: ", err)
		}
	}

	skipSetup := false
	if val, ok := s.Var("tast_skip_setup_check"); ok {
		var err error
		if skipSetup, err = strconv.ParseBool(val); err != nil {
			s.Fatal("Cannot parse argument 'tast_skip_setup_check' of type bool: ", err)
		}
	}

	// Before running any functional test block, test setup should be validated, unless bypassed.
	passed := skipSetup
	if !passed {
		passed = s.Run(ctx, "setup_checks", func(ctx context.Context, s *testing.State) {
			util.SetupChecks(ctx, s)
		})
	}
	if passed {
		passed = s.Run(ctx, "storage_subtest", func(ctx context.Context, s *testing.State) {
			resultWriter := &util.FioResultWriter{}
			defer resultWriter.Save(ctx, s.OutDir(), true)
			subtest(ctx, s, resultWriter, testParam)
		})
	}
	if err := util.WriteTestStatusFile(ctx, s.OutDir(), passed, start); err != nil {
		s.Fatal("Error writing status file: ", err)
	}
}
