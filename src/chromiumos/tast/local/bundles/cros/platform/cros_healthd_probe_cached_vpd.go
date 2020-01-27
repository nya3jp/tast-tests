// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"

	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeCachedVpd,
		Desc: "Check that we can probe cros_healthd for cached vpd info",
		Contacts: []string{
			"jschettler@google.com",
			"khegde@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cros_config", "diagnostics"},
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

	// Check if the device has a SKU number. If it does, the SKU number should
	// be printed. If it does not, "NA" should be printed.
	hasSku, err := crosconfig.Get(ctx, "/cros-healthd/cached-vpd", "has-sku-number")
	if err != nil {
		s.Fatal("Failed to get has-sku-number property: ", err)
	}

	sku := lines[1]
	if hasSku == "true" && sku == "" {
		s.Fatal("Empty SKU number")
	}

	if hasSku != "true" && sku != "NA" {
		s.Fatalf("Incorrect SKU number: got %v, want NA", sku)
	}
}
