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
	"chromiumos/tast/errors"
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

func checkVulkan(ctx context.Context, s *testing.State, cmd *testexec.Cmd, vulkanEnabled bool) (*version, error) {
	out, err := cmd.CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		if !vulkanEnabled {
			// Expected. No need to continue running.
			testing.ContextLog(ctx, "Expected vulkaninfo failure: ", err)
			return &version{0, 0, 0}, nil
		}
		return nil, errors.Wrap(err, "failed to run vulkaninfo")
	}

	// Write file to vulkaninfo.txt
	infoFile := filepath.Join(s.OutDir(), "vulkaninfo.txt")
	file, err := os.Create(infoFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create file: %s", infoFile)
	}
	defer file.Close()
	if _, err := file.Write(out); err != nil {
		return nil, errors.Wrap(err, "failed to write vulkaninfo")
	}

	if !vulkanEnabled {
		return nil, errors.New("expected disabled vulkan")
	}

	// Update instanceVersion.
	instanceVersion := version{0, 0, 0}
	instanceRE := regexp.MustCompile(`Vulkan Instance Version: (\d+).(\d+).(\d+)`)
	matches := instanceRE.FindStringSubmatch(string(out))
	if matches == nil {
		return nil, errors.New("failed to get vulkan instance version")
	}
	convertInt := func(str string) (uint64, error) {
		i, err := strconv.ParseUint(str, 10, 64)
		if err != nil {
			return 0, errors.Errorf("failed to parse %s", str)
		}
		return i, nil
	}

	var major uint64
	var minor uint64
	var patch uint64
	if major, err = convertInt(matches[1]); err != nil {
		return nil, errors.Wrap(err, "failed to parse major number")
	}
	if minor, err = convertInt(matches[2]); err != nil {
		return nil, errors.Wrap(err, "failed to parse minor number")
	}
	if patch, err = convertInt(matches[3]); err != nil {
		return nil, errors.Wrap(err, "failed to parse patch number")
	}
	instanceVersion.major = major
	instanceVersion.minor = minor
	instanceVersion.patch = patch
	return &instanceVersion, nil
}

// VKInfo checks if vulkan is available in the DUT.
func VKInfo(ctx context.Context, s *testing.State) {
	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()
	vulkanEnabled := s.Param().(bool)
	cmd := testexec.CommandContext(ctx, "vulkaninfo")
	v, err := checkVulkan(ctx, s, cmd, vulkanEnabled)
	if err != nil {
		s.Fatal("Failed to verify vulkaninfo: ", err)
	}
	v.setPerf(pv)
}
