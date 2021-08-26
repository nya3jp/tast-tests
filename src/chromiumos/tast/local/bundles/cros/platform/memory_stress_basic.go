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
	"chromiumos/tast/local/bundles/cros/platform/memorystress"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

type testParams struct {
	isLacros bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     MemoryStressBasic,
		Desc:     "Create heavy memory pressure and check if oom-killer is invoked",
		Contacts: []string{"vovoy@chromium.org", "chromeos-memory@google.com"},
		// This test takes 15-30 minutes to run.
		Timeout: 45 * time.Minute,
		Data: []string{
			memorystress.AllocPageFilename,
			memorystress.JavascriptFilename,
		},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			"platform.MemoryStressBasic.enableARC",
			"platform.MemoryStressBasic.minFilelistKB",
			"platform.MemoryStressBasic.seed",
			"platform.MemoryStressBasic.useHugePages",
		},
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_memory_nightly"},
			ExtraSoftwareDeps: []string{"android_p"},
			Val: testParams{
				isLacros: false,
			},
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_memory_nightly"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testParams{
				isLacros: false,
			},
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "lacros",
			Val: testParams{
				isLacros: true,
			},
		}},
	})

}

func MemoryStressBasic(ctx context.Context, s *testing.State) {
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

	baseURL := server.URL + "/" + memorystress.AllocPageFilename

	perfValues := perf.NewValues()

	const mbPerTab = 800
	if s.Param().(testParams).isLacros {
		if err := lacrosMain(ctx, s, localRand, mbPerTab, baseURL, perfValues); err != nil {
			s.Fatal("lacrosMain failed: ", err)
		}
	} else {
		if err := stressMain(ctx, localRand, mbPerTab, minFilelistKB, baseURL, enableARC, useHugePages, perfValues); err != nil {
			s.Fatal("stressMain failed: ", err)
		}
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

func stressMain(ctx context.Context, localRand *rand.Rand, mbPerTab, minFilelistKB int, baseURL string, enableARC, useHugePages bool, perfValues *perf.Values) error {
	// Tests both the low compress ratio and high compress ratio cases.
	// When there is more random data (67 percent random), the compress ratio is low,
	// the low memory notification is triggered by low uncompressed anonymous memory.
	// When there is less random data (33 percent random), the compress ratio is high,
	// the low memory notification is triggered by low swap free.
	const switchCount = 150
	result67, err := stressTestCase(ctx, localRand, mbPerTab, switchCount, minFilelistKB, 0.67, baseURL, enableARC, useHugePages)
	if err != nil {
		return errors.Wrap(err, "67_percent_random test case failed")
	}
	result33, err := stressTestCase(ctx, localRand, mbPerTab, switchCount, minFilelistKB, 0.33, baseURL, enableARC, useHugePages)
	if err != nil {
		return errors.Wrap(err, "33_percent_random test case failed")
	}

	if err := memorystress.ReportTestCaseResult(ctx, perfValues, result67, "67_percent_random"); err != nil {
		return errors.Wrap(err, "reporting 67_percent_random failed")
	}
	if err := memorystress.ReportTestCaseResult(ctx, perfValues, result33, "33_percent_random"); err != nil {
		return errors.Wrap(err, "reporting 33_percent_random failed")
	}

	return nil
}

func stressTestCase(ctx context.Context, localRand *rand.Rand, mbPerTab, switchCount, minFilelistKB int, compressRatio float64, baseURL string, enableARC, useHugePages bool) (memorystress.TestCaseResult, error) {
	var opts []chrome.Option
	if enableARC {
		opts = append(opts, chrome.ARCEnabled())
	}
	if useHugePages {
		opts = append(opts, chrome.HugePagesEnabled())
	}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return memorystress.TestCaseResult{}, errors.Wrap(err, "cannot start chrome")
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return memorystress.TestCaseResult{}, errors.Wrap(err, "failed to wait for idle CPU")
	}

	// Setup min_filelist_kbytes after chrome start.
	if minFilelistKB >= 0 {
		if err := ioutil.WriteFile("/proc/sys/vm/min_filelist_kbytes", []byte(strconv.Itoa(minFilelistKB)), 0644); err != nil {
			return memorystress.TestCaseResult{}, errors.Wrap(err, "could not write to /proc/sys/vm/min_filelist_kbytes")
		}
	}

	return memorystress.TestCase(ctx, cr, localRand, mbPerTab, switchCount, compressRatio, baseURL)
}

func lacrosMain(ctx context.Context, s *testing.State, localRand *rand.Rand, mbPerTab int, baseURL string, perfValues *perf.Values) error {
	// TODO(b/191105438): Tune Lacros variation when Lacros tab discarder is mature.
	lacros, err := launcher.LaunchLacrosChrome(ctx, s.FixtValue().(launcher.FixtData))
	if err != nil {
		return errors.Wrap(err, "failed to launch lacros-chrome")
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for idle CPU")
	}

	const switchCount = 10
	const compressRatio = 0.67
	result, err := memorystress.TestCase(ctx, lacros, localRand, mbPerTab, switchCount, compressRatio, baseURL)
	if err != nil {
		return errors.Wrap(err, "memorystress test case failed")
	}

	if err := memorystress.ReportTestCaseResult(ctx, perfValues, result, "stress"); err != nil {
		return errors.Wrap(err, "reporting result failed")
	}

	return nil
}
