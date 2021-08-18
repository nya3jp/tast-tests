// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/memory/mempressure"
	"chromiumos/tast/testing"
)

type memoryPressureParams struct {
	enableARC    bool
	useHugePages bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     MemoryPressure,
		Desc:     "Create memory pressure and collect various measurements from Chrome and from the kernel",
		Contacts: []string{"semenzato@chromium.org", "sonnyrao@chromium.org", "chromeos-memory@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_memory_nightly"},
		Timeout:  180 * time.Minute,
		Data: []string{
			mempressure.CompressibleData,
			mempressure.WPRArchiveName,
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: &memoryPressureParams{enableARC: false, useHugePages: false},
		}, {
			Name:              "vm",
			Val:               &memoryPressureParams{enableARC: true, useHugePages: false},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "huge_pages_vm",
			Val:               &memoryPressureParams{enableARC: true, useHugePages: true},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "container",
			Val:               &memoryPressureParams{enableARC: true, useHugePages: false},
			ExtraSoftwareDeps: []string{"android_p"},
		}},
	})
}

// MemoryPressure is the main test function.
func MemoryPressure(ctx context.Context, s *testing.State) {
	enableARC := s.Param().(*memoryPressureParams).enableARC
	useHugePages := s.Param().(*memoryPressureParams).useHugePages

	testEnv, err := mempressure.NewTestEnv(ctx, s.OutDir(), enableARC, useHugePages, s.DataPath(mempressure.WPRArchiveName))
	if err != nil {
		s.Fatal("Failed creating the test environment: ", err)
	}
	defer testEnv.Close(ctx)

	p := &mempressure.RunParameters{
		PageFilePath:             s.DataPath(mempressure.CompressibleData),
		PageFileCompressionRatio: 0.40,
	}

	if err := mempressure.Run(ctx, s.OutDir(), testEnv.Chrome(), testEnv.ARC(), p); err != nil {
		s.Fatal("Run failed: ", err)
	}
}
