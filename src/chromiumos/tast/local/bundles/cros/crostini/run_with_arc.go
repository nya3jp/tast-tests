// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RunWithARC,
		Desc:         "Checks that ARC(VM) runs in parallel with Crostini",
		Contacts:     []string{"niwa@chromium.org", "arcvm-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedARCEnabled(),
		SoftwareDeps: []string{"chrome", "vm_host", "android_both"},
	})
}

func RunWithARC(ctx context.Context, s *testing.State) {
	// First ensure crostini works in isolation by running their sanity test.
	Sanity(ctx, s)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	// Ensures package manager service is running by checking the existence of
	// "android" package.
	out, err := a.Command(ctx, "pm", "list", "packages").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("pm list failed: ", err)
	}

	pkgs := strings.Split(string(out), "\n")
	found := false
	for _, p := range pkgs {
		if p == "package:android" {
			found = true
			break
		}
	}

	if !found {
		s.Fatal("android package not found: ", pkgs)
	}
}
