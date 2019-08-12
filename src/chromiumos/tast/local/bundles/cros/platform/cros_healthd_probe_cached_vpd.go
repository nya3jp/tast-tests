// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeCachedVpd,
		Desc: "Check that we can probe cros_healthd for cached vpd info",
		Contacts: []string{
			"khegde@google.com",
			"pmoy@google.com",
			"sjg@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func CrosHealthdProbeCachedVpd(ctx context.Context, s *testing.State) {
	b, err := testexec.CommandContext(ctx, "cros_healthd", "--probe_cached_vpd").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run 'cros_healthd --probe_cached_vpd': ", err)
	}
	tokens := strings.Split(string(b), ":")
	badOutput := len(tokens) != 2 || tokens[0] != "sku_number" || len(tokens[1]) == 0
	if badOutput {
		s.Fatalf("Received bad output: got %q; want %q", string(b), "sku_number: $SKU_NUMBER\n")
	}
}
