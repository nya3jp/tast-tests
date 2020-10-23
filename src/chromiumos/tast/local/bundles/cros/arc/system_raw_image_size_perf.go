// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     SystemRawImageSizePerf,
		Desc:     "Outputs the size of ARC {system,vendor}.raw.img and the rootfs",
		Contacts: []string{"bhansknecht@chromium.org", "arc-eng@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func SystemRawImageSizePerf(ctx context.Context, s *testing.State) {
	systemSizeBytes, err := testexec.CommandContext(ctx, "stat", "-c", "%s", "/opt/google/containers/android/system.raw.img").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get system.raw.img size: ", err)
	}
	systemSizeString := strings.TrimSpace(string(systemSizeBytes))
	systemSize, err := strconv.ParseFloat(systemSizeString, 64)
	if err != nil {
		s.Fatal("Failed to parse system.raw.img size: ", err)
	}

	vendorSizeBytes, err := testexec.CommandContext(ctx, "stat", "-c", "%s", "/opt/google/containers/android/vendor.raw.img").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get vendor.raw.img size: ", err)
	}
	vendorSizeString := strings.TrimSpace(string(vendorSizeBytes))
	vendorSize, err := strconv.ParseFloat(vendorSizeString, 64)
	if err != nil {
		s.Fatal("Failed to parse vendor.raw.img size: ", err)
	}

	dfBytes, err := testexec.CommandContext(ctx, "df", "/").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get rootfs size: ", err)
	}
	// Sample df output:
	// Filesystem     1K-blocks    Used Available Use% Mounted on
	// /dev/root            XXX     YYY  ...
	// Report XXX and YYY as scores.
	dfFields := strings.Fields(string(dfBytes))
	rootfsTotal, err := strconv.ParseFloat(dfFields[8], 64)
	if err != nil {
		s.Fatal("Failed to parse rootfs total size: ", err)
	}
	rootfsUsed, err := strconv.ParseFloat(dfFields[9], 64)
	if err != nil {
		s.Fatal("Failed to parse rootfs used size: ", err)
	}

	p := perf.NewValues()
	p.Set(perf.Metric{
		Name:      "bytes_system_raw_img",
		Unit:      "bytes",
		Direction: perf.SmallerIsBetter,
	}, systemSize)
	p.Set(perf.Metric{
		Name:      "bytes_vendor_raw_img",
		Unit:      "bytes",
		Direction: perf.SmallerIsBetter,
	}, vendorSize)
	p.Set(perf.Metric{
		Name:      "bytes_system_vendor_total",
		Unit:      "bytes",
		Direction: perf.SmallerIsBetter,
	}, systemSize+vendorSize)
	p.Set(perf.Metric{
		Name:      "bytes_rootfs_total",
		Unit:      "bytes",
		Direction: perf.SmallerIsBetter,
	}, 1024*rootfsTotal)
	p.Set(perf.Metric{
		Name:      "bytes_rootfs_used",
		Unit:      "bytes",
		Direction: perf.SmallerIsBetter,
	}, 1024*rootfsUsed)

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
