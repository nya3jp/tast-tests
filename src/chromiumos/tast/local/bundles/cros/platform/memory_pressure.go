// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/memory/mempressure"
	"chromiumos/tast/testing"
)

type memoryPressureParams struct {
	enableARC    bool
	useHugePages bool
	bt           browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MemoryPressure,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Create memory pressure and collect various measurements from Chrome and from the kernel",
		Contacts:     []string{"bgeffon@chromium.org", "vovoy@chromium.org", "chromeos-memory@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_memory_nightly"},
		Timeout:      180 * time.Minute,
		Data: []string{
			mempressure.CompressibleData,
			mempressure.WPRArchiveName,
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: memoryPressureParams{enableARC: false, useHugePages: false, bt: browser.TypeAsh},
		}, {
			Name:              "vm",
			Val:               memoryPressureParams{enableARC: true, useHugePages: false, bt: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "huge_pages_vm",
			Val:               memoryPressureParams{enableARC: true, useHugePages: true, bt: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "container",
			Val:               memoryPressureParams{enableARC: true, useHugePages: false, bt: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "lacros",
			Val:               memoryPressureParams{enableARC: false, useHugePages: false, bt: browser.TypeLacros},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

// MemoryPressure is the main test function.
func MemoryPressure(ctx context.Context, s *testing.State) {
	enableARC := s.Param().(memoryPressureParams).enableARC
	useHugePages := s.Param().(memoryPressureParams).useHugePages
	bt := s.Param().(memoryPressureParams).bt

	testEnv, err := mempressure.NewTestEnv(ctx, s.OutDir(), enableARC, useHugePages, bt, s.DataPath(mempressure.WPRArchiveName))
	if err != nil {
		s.Fatal("Failed creating the test environment: ", err)
	}
	defer testEnv.Close(ctx)

	p := &mempressure.RunParameters{
		PageFilePath:             s.DataPath(mempressure.CompressibleData),
		PageFileCompressionRatio: 0.40,
	}

	if err := mempressure.Run(ctx, s.OutDir(), testEnv.Browser(), testEnv.ARC(), p); err != nil {
		s.Fatal("Run failed: ", err)
	}
}
