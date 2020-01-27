// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeCachedVpd,
		Desc: "Check that we can probe cros_healthd for cached vpd info",
		Contacts: []string{
			"khegde@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func CrosHealthdProbeCachedVpd(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		s.Fatal("Failed to start cros_healthd: ", err)
	}
	b, err := testexec.CommandContext(ctx, "telem", "--category=cached_vpd").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run 'telem --category=cached_vpd': ", err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) != 2 {
		s.Fatalf("Wrong number of lines: got %v, want 2", len(lines))
	}
	// Expect the keys to be just sku_number.
	want := "sku_number"
	got := lines[0]
	if !(want == got) {
		s.Fatalf("Header keys: got %v; want %v", got, want)
	}
	// Check if the device has a SKU number. If it does, the SKU number should be printed. If it does not, "NA" should be printed.
	b, err = testexec.CommandContext(ctx, "cros_config", "/cros-healthd/cached-vpd", "has-sku-number").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run 'cros_config /cros-healthd/cached-vpd has-sku-number': ", err)
	}
	val := strings.TrimSpace(string(b))
	sku := lines[1]
	if val == "true" && sku == "" {
		s.Fatal("Empty sku_number")
	}
	if val != "true" && sku != "NA" {
		s.Fatal("sku_number should be NA")
	}
}
