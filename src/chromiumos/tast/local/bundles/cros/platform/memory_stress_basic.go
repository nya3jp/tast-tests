// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/memory/stress"
	"chromiumos/tast/testing"
)

type testCaseResult = stress.TestCaseResult

const (
	allocPageFilename  = "memory_stress.html"
	javascriptFilename = "memory_stress.js"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MemoryStressBasic,
		Desc:     "Create heavy memory pressure and check if oom-killer is invoked",
		Contacts: []string{"vovoy@chromium.org", "chromeos-memory@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_memory_nightly"},
		// This test takes 15-30 minutes to run.
		Timeout: 45 * time.Minute,
		Data: []string{
			allocPageFilename,
			javascriptFilename,
		},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			"platform.MemoryStressBasic.enableARC",
			"platform.MemoryStressBasic.minFilelistKB",
			"platform.MemoryStressBasic.seed",
			"platform.MemoryStressBasic.useHugePages",
		},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})

}

func MemoryStressBasic(ctx context.Context, s *testing.State) {
	const (
		mbPerTab    = 800
		switchCount = 150
	)

	minFilelistKB := -1
	if val, ok := s.Var("platform.MemoryStressBasic.minFilelistKB"); ok {
		val, err := strconv.Atoi(val)
		if err != nil {
			s.Fatal("Cannot parse argument platform.MemoryStressBasic.minFilelistKB: ", err)
		}
		minFilelistKB = val
	}
	s.Log(ctx, "minFilelistKB: ", minFilelistKB)

	// The memory pressure is higher when ARC is enabled (without launching Android apps).
	// Checks the ARC enabled case by default.
	enableARC := true
	if val, ok := s.Var("platform.MemoryStressBasic.enableARC"); ok {
		boolVal, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Cannot parse argument platform.MemoryStressBasic.enableARC: ", err)
		}
		enableARC = boolVal
	}
	s.Log(ctx, "enableARC: ", enableARC)

	useHugePages := false
	if val, ok := s.Var("platform.MemoryStressBasic.useHugePages"); ok {
		boolVal, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Cannot parse argument platform.MemoryStressBasic.useHugePages: ", err)
		}
		useHugePages = boolVal
	}
	s.Log(ctx, "useHugePages: ", useHugePages)

	seed := time.Now().UTC().UnixNano()
	if val, ok := s.Var("platform.MemoryStressBasic.seed"); ok {
		intVal, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			s.Fatal("Cannot parse argument platform.MemoryStressBasic.seed: ", err)
		}
		seed = intVal
	}
	s.Log(ctx, "Seed: ", seed)
	localRand := rand.New(rand.NewSource(seed))

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	baseURL := server.URL + "/" + allocPageFilename

	perfValues := perf.NewValues()

	// Tests both the low compress ratio and high compress ratio cases.
	// When there is more random data (67 percent random), the compress ratio is low,
	// the low memory notification is triggered by low uncompressed anonymous memory.
	// When there is less random data (33 percent random), the compress ratio is high,
	// the low memory notification is triggered by low swap free.
	label67 := "67_percent_random"
	result67, err := stressTestCase(ctx, localRand, mbPerTab, switchCount, minFilelistKB, 0.67, baseURL, label67, enableARC, useHugePages)
	if err != nil {
		s.Fatal("67_percent_random test case failed: ", err)
	}
	label33 := "33_percent_random"
	result33, err := stressTestCase(ctx, localRand, mbPerTab, switchCount, minFilelistKB, 0.33, baseURL, label33, enableARC, useHugePages)
	if err != nil {
		s.Fatal("33_percent_random test case failed: ", err)
	}

	if err := stress.ReportTestCaseResult(ctx, perfValues, result67, label67); err != nil {
		s.Fatal("Reporting 67_percent_random failed: ", err)
	}
	if err := stress.ReportTestCaseResult(ctx, perfValues, result33, label33); err != nil {
		s.Fatal("Reporting 33_percent_random failed: ", err)
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

func stressTestCase(ctx context.Context, localRand *rand.Rand, mbPerTab, switchCount, minFilelistKB int, compressRatio float64, baseURL, label string, enableARC, useHugePages bool) (testCaseResult, error) {
	var opts []chrome.Option
	if enableARC {
		opts = append(opts, chrome.ARCEnabled())
	}
	if useHugePages {
		opts = append(opts, chrome.HugePagesEnabled())
	}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return testCaseResult{}, errors.Wrap(err, "cannot start chrome")
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to wait for idle CPU")
	}

	// Setup min_filelist_kbytes after chrome start.
	if minFilelistKB >= 0 {
		if err := ioutil.WriteFile("/proc/sys/vm/min_filelist_kbytes", []byte(strconv.Itoa(minFilelistKB)), 0644); err != nil {
			return testCaseResult{}, errors.Wrap(err, "could not write to /proc/sys/vm/min_filelist_kbytes")
		}
	}

	return stress.TestCase(ctx, cr, localRand, mbPerTab, switchCount, compressRatio, baseURL)
}
