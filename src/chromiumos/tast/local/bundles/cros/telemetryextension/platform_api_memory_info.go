// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"
	"io/ioutil"
	"regexp"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/bundles/cros/telemetryextension/fixture"
	"chromiumos/tast/testing"
)

var (
	memTotalRegexp  = regexp.MustCompile("MemTotal: +([0-9]+) kB")
	pageFaultRegexp = regexp.MustCompile("pgfault ([0-9]+)")
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlatformAPIMemoryInfo,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests chrome.os.telemetry.getMemoryInfo Chrome Extension API function exposed to Telemetry Extension",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "stable",
				Fixture:           fixture.TelemetryExtension,
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable",
				Fixture:           fixture.TelemetryExtension,
				ExtraHardwareDeps: dep.NonStableModels(),
			},
			{
				Name:              "stable_lacros",
				Fixture:           fixture.TelemetryExtensionLacros,
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable_lacros",
				Fixture:           fixture.TelemetryExtensionLacros,
				ExtraHardwareDeps: dep.NonStableModels(),
			},
		},
	})
}

// PlatformAPIMemoryInfo tests chrome.os.telemetry.getMemoryInfo Chrome Extension API functionality.
func PlatformAPIMemoryInfo(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	wantTotalMemory, err := fetchIntFromFile("/proc/meminfo", memTotalRegexp)
	if err != nil {
		s.Fatal("Failed to fetch total memory: ", err)
	}

	wantPageFaults, err := fetchIntFromFile("/proc/vmstat", pageFaultRegexp)
	if err != nil {
		s.Fatal("Failed to fetch total memory: ", err)
	}

	type response struct {
		TotalMemoryKiB          int64 `json:"totalMemoryKiB"`
		FreeMemoryKiB           int64 `json:"freeMemoryKiB"`
		AvailableMemoryKiB      int64 `json:"availableMemoryKiB"`
		PageFaultsSinceLastBoot int64 `json:"pageFaultsSinceLastBoot"`
	}

	var resp response
	if err := v.ExtConn.Call(ctx, &resp,
		"tast.promisify(chrome.os.telemetry.getMemoryInfo)",
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	if resp.TotalMemoryKiB != wantTotalMemory {
		s.Errorf("Unexpecteed total memory: got %d; want %d", resp.TotalMemoryKiB, wantTotalMemory)
	}

	if resp.FreeMemoryKiB <= 0 {
		s.Errorf("Unexpecteed free memory: got %d; want >0", resp.FreeMemoryKiB)
	}

	if resp.AvailableMemoryKiB <= 0 {
		s.Errorf("Unexpecteed available memory: got %d; want >0", resp.AvailableMemoryKiB)
	}

	if resp.PageFaultsSinceLastBoot < wantPageFaults {
		s.Errorf("Unexpecteed total memory: got %d; want >=%d", resp.PageFaultsSinceLastBoot, wantPageFaults)
	}
}

func fetchIntFromFile(filePath string, re *regexp.Regexp) (int64, error) {
	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read file")
	}

	m := re.FindStringSubmatch(string(b))
	if len(m) != 2 {
		return 0, errors.Errorf("unexpected match (%q) size = got %d; want %d", m, len(m), 2)
	}

	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to convert %q to number", string(m[1]))
	}
	return n, nil
}
