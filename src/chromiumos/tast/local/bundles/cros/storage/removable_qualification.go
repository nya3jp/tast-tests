// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
		Func:         RemovableQualification,
		Desc:         "Performs a storage qualification test for removable devices",
		Contacts:     []string{"chromeos-engprod-platform-syd@google.com"},
		Attr:         []string{"group:storage-qual"},
		Data:         util.Configs,
		SoftwareDeps: []string{"storage_wearout_detect"},
		Vars:         []string{"tast_suspend_block_timeout", "tast_skip_setup_check", "tast_skip_s0ix_check"},
		Params: []testing.Param{{
			Name:    "setup_benchmarks",
			Val:     util.SetupBenchmarks,
			Timeout: 1 * time.Hour,
		}, {
			Name:    "functional",
			Val:     util.RemovableRunner,
			Timeout: 2 * time.Hour,
		}, {
			Name:    "teardown_benchmarks",
			Val:     util.SetupBenchmarks,
			Timeout: 1 * time.Hour,
		}},
	})
}

// RemovableQualification runs a disk IO qualification test for removable storage.
// The test runs some performance tests as well as suspend functionality.
func RemovableQualification(ctx context.Context, s *testing.State) {
	subtest := s.Param().(func(context.Context, *testing.State, *util.FioResultWriter, util.QualParam))
	start := time.Now()

	testParam := util.QualParam{}

	testParam.SuspendBlockTimeout = util.DefaultSuspendBlockTimeout
	testParam.SkipS0iXResidencyCheck = false
	var err error
	if testParam.TestDevice, err = util.RemovableDevice(ctx); err != nil {
		s.Fatal("Failed to get removable device: ", err)
	}

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

	passed := s.Run(ctx, "storage_subtest", func(ctx context.Context, s *testing.State) {
		resultWriter := &util.FioResultWriter{}
		defer resultWriter.Save(ctx, s.OutDir(), true)
		subtest(ctx, s, resultWriter, testParam)
	})
	if err := util.WriteTestStatusFile(ctx, s.OutDir(), passed, start); err != nil {
		s.Fatal("Error writing status file: ", err)
	}

}
