// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package system contains local Tast tests that exercise system configuration.
package system

import (
	"context"
	"strconv"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome/chromeproc"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Version,
		Desc: "Reports various component versions from the system image",
		Contacts: []string{
			"pwang@chromium.org",
		},
		Attr: []string{"group:mainline", "informational", "group:graphics", "graphics_perbuild"},
	})
}

// Version checks if the version can be correctly extracted.
func Version(ctx context.Context, s *testing.State) {
	lsb, err := lsbrelease.Load()
	if err != nil {
		s.Fatal("Failed to read lsbrelease: ", err)
	}

	// We report these versions as a perf values such that services, e.g. bisector, can then use these values as an integration check.
	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()

	convertAndSave := func(str, name string, pv *perf.Values) {
		number, err := strconv.Atoi(str)
		if err != nil {
			s.Errorf("Failed to convert %s to integer: %v", str, err)
		} else {
			pv.Set(perf.Metric{
				Name:      name,
				Unit:      "version",
				Direction: perf.SmallerIsBetter,
			}, float64(number))
		}
	}

	// Report the ChromeOS build version.
	if buildNumber, ok := lsb[lsbrelease.BuildNumber]; !ok {
		s.Error("Failed to get ChromeOS Build number")
	} else {
		convertAndSave(buildNumber, "CHROMEOS_BUILD", pv)
	}

	// Report the Chrome version.
	if chromeVersion, err := chromeproc.Version(ctx); err != nil {
		s.Error("Failed to get Chrome version: ", err)
	} else {
		s.Log("chromeVersion: ", chromeVersion)
		// ChromeVersion consists of 4 digits. The second value is always zero.
		convertAndSave(chromeVersion[0], "CHROME_MILESTONE", pv)
		convertAndSave(chromeVersion[2], "CHROME_BUILD", pv)
		convertAndSave(chromeVersion[3], "CHROME_PATCH", pv)
	}

	// Report the ARC version.
	if ARCVersion, ok := lsb[lsbrelease.ARCVersion]; !ok {
		// Make it pass as old devices may not have ARC version.
		s.Log("ARC_VERSION is not exist in lsb-release")
	} else {
		convertAndSave(ARCVersion, "ARC_VERSION", pv)
	}
}
