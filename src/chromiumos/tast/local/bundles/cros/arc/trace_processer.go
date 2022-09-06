// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TraceProcesser,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Execute trace_processor",
		Contacts:     []string{"sstan@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      "arcBootedInClamshellMode",
		Timeout:      4 * time.Minute,
	})
}

func TraceProcesser(ctx context.Context, s *testing.State) {
	if stdout, stderr, err := testexec.CommandContext(ctx, "curl", "-LO", "https://get.perfetto.dev/trace_processor").SeparatedOutput(); err != nil {
		s.Error("Cannot cmd #1", err)
	} else {
		s.Logf("Stdout#1: %v, Stderr#1: %v", stdout, string(stderr[:]))
	}

	if stdout, stderr, err := testexec.CommandContext(ctx, "chmod", "+x", "trace_processor").SeparatedOutput(); err != nil {
		s.Error("Cannot cmd #2", err)
	} else {
		s.Logf("Stdout#2: %v, Stderr#2: %v", stdout, string(stderr[:]))
	}

	if stdout, stderr, err := testexec.CommandContext(ctx, "./trace_processor", "--help").SeparatedOutput(); err != nil {
		s.Error("Cannot cmd #3", err)
	} else {
		s.Logf("Stdout#3: %v, Stderr#3: %v", stdout, string(stderr[:]))
	}
}
