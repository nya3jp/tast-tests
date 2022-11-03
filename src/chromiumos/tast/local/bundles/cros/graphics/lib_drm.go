// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

var (
	errs       = []string{}
	testCommon = []string{
		"modetest",
	}
	testsExynos = []string{
		"kmstest",
	}
	testsMediaTek = []string{
		"kmstest",
	}
	testsQualcomm = []string{
		"kmstest",
	}
	testsRockchip = []string{
		"kmstest",
	}
	archTests = map[string][]string{
		"amd":      []string{},
		"arm":      []string{},
		"exynos5":  testsExynos,
		"i386":     []string{},
		"mediatek": testsMediaTek,
		"qualcomm": testsQualcomm,
		"rockchip": testsRockchip,
		"tegra":    []string{},
		"x86_64":   []string{},
	}
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LibDRM,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Run binary test(s) and verify that no errors are encountered",
		// TODO(syedfaaiz): Add to CQ once it is green and stable.
		Attr: []string{"group:graphics", "graphics_nightly"},
		Contacts: []string{"syedfaaiz@google.com",
			"chromeos-gfx@google.com",
		},
		Fixture: "gpuWatchDog",
		Timeout: 2 * time.Minute,
	})
}

func LibDRM(ctx context.Context, s *testing.State) {
	_, soc, err := sysutil.KernelVersionAndArch()
	if err != nil {
		s.Fatal("Error while fetching CPU family : ", err)
	}
	if _, exists := archTests[soc]; !exists {
		s.Fatalf("Error: Architecture %s not supported", soc)
	}
	if soc == "tegra" {
		testing.ContextLog(ctx, "Tegra does not support DRM")
		return
	}
	tests := append(testCommon, archTests[soc]...)
	// Stop the UI, be sure to start it again later.
	_, stderr, err := testexec.CommandContext(ctx, "stop", "ui").SeparatedOutput(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("Error failed to stop UI : %s", string(stderr))
	}
	for _, test := range tests {
		_, _, err := testexec.CommandContext(ctx, "which", test).SeparatedOutput(testexec.DumpLogOnError)
		if err != nil {
			testerr := strings.Join([]string{test, "was not found"}, " ")
			errs = append(errs, testerr)
		} else {
			_, stderr, err := testexec.CommandContext(ctx, test).SeparatedOutput(testexec.DumpLogOnError)
			if err != nil {
				testerr := strings.Join([]string{test, "failed with error", string(stderr)}, " ")
				errs = append(errs, testerr)
			}
		}
	}
	// Start the UI again.
	_, _, err = testexec.CommandContext(ctx, "start", "ui").SeparatedOutput(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("Error failed to start UI : %s", string(stderr))
	}
	if len(errs) > 0 {
		outErr := strings.Join(errs, "\n")
		s.Fatalf("The following errors were encountered :%s", outErr)
	}
}
