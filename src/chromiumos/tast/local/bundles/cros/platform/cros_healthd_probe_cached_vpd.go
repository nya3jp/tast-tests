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
	if len(tokens) != 2 {
		s.Fatalf("Found wrong number of tokens in output: got %v, want %v", len(tokens), 2)
	}
	if tokens[0] != "sku_number" {
		s.Fatalf("Found incorrect first token: got %q, want %q", tokens[0], "sku_number")
	}
	if len(tokens[1]) == 0 {
		s.Fatal("Got: empty sku_number, Want: nonempty sku number")
	}
}
