// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"bufio"
	"context"
	"io"
	"strings"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RunTestsKillStaleBundles,
		Desc:     "Verifies that Tast run kills already running bundles",
		Contacts: []string{"oka@chromium.org", "tast-core@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func RunTestsKillStaleBundles(ctx context.Context, s *testing.State) {
	cmd, err := tastrun.NewCommand(ctx, s, "run", nil, []string{"meta.LocalFreezeForever"})
	if err != nil {
		s.Fatal("Creating command to run tast: ", err)
	}
	r, w := io.Pipe()
	cmd.Stdout = w

	s.Log("Starting ", strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		s.Fatal("Starting tast: ", err)
	}
	sc := bufio.NewScanner(r)

	started := make(chan struct{})
	go func() {
		for sc.Scan() {
			line := sc.Text()
			s.Log(line)
			if strings.Contains(line, "LocalFreezeForever started") {
				close(started)
			}
		}
	}()

	<-started
	_, _, err = tastrun.Exec(ctx, s, "run", nil, []string{"example.Pass"})
	if err != nil {
		s.Fatal("Second tast run failed: ", err)
	}

	// Already running bundle should be killed when a new bundle is run, and
	// tast should terminate.
	cmd.Wait()
}
