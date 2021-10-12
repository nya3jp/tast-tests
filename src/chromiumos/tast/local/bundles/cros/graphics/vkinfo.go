// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: VKInfo,
		Desc: "Quick smoke check for Vulkan",
		Contacts: []string{
			"pwang@chromium.org",
			"chromeos-gfx@chromium.org",
		},
		Attr: []string{"group:mainline", "informational", "group:graphics", "graphics_perbuild"},
		Params: []testing.Param{{
			Name:              "",
			ExtraSoftwareDeps: []string{"vulkan"},
			Val:               true,
		}, {
			Name:              "no_vulkan",
			ExtraSoftwareDeps: []string{"no_vulkan"},
			Val:               false,
		}},
	})
}

type version struct {
	major uint64
	minor uint64
	patch uint64
}

func (v *version) setPerf(pv *perf.Values) {
	pv.Set(perf.Metric{
		Name:      "major",
		Unit:      "version",
		Direction: perf.BiggerIsBetter,
	}, float64(v.major))
	pv.Set(perf.Metric{
		Name:      "minor",
		Unit:      "version",
		Direction: perf.BiggerIsBetter,
	}, float64(v.minor))
	pv.Set(perf.Metric{
		Name:      "patch",
		Unit:      "version",
		Direction: perf.BiggerIsBetter,
	}, float64(v.patch))
}

// VKInfo checks if vulkan is available in the DUT.
func VKInfo(ctx context.Context, s *testing.State) {
	pv := perf.NewValues()
	instanceVersion := version{0, 0, 0}
	defer func() {
		instanceVersion.setPerf(pv)
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()

	vulkanEnabled := s.Param().(bool)
	cmd := testexec.CommandContext(ctx, "vulkaninfo")
	out, err := cmd.CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		if vulkanEnabled == false {
			// Expected. No need to continue running.
			return
		}
		s.Fatal("Failed to run vulkaininfo: ", err)
	}
	if vulkanEnabled == false {
		s.Fatal("Expected vulkan is disabled on DUT")
	}

	// Write file to vulkaninfo.txt
	infoFile := filepath.Join(s.OutDir(), "vulkaninfo.txt")
	file, err := os.Create(infoFile)
	if err != nil {
		s.Fatalf("Failed to create file: %s: %v", infoFile, err)
	}
	defer file.Close()
	if _, err := file.Write(out); err != nil {
		s.Fatal("Failed to write vulkaninfo: ", err)
	}

	// Update instanceVersion.
	instanceRE := regexp.MustCompile(`Vulkan Instance Version: (\d+).(\d+).(\d+)`)
	matches := instanceRE.FindStringSubmatch(string(out))
	if matches == nil {
		s.Fatal("Failed to get vulkan instance version")
	}
	convertInt := func(str string) uint64 {
		i, err := strconv.ParseUint(str, 10, 64)
		if err != nil {
			s.Fatal("Failed to parse ", s)
		}
		return i
	}
	instanceVersion.major = convertInt(matches[1])
	instanceVersion.minor = convertInt(matches[2])
	instanceVersion.patch = convertInt(matches[3])
}
