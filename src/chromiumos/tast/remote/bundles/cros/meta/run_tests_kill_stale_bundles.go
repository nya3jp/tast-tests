// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"bufio"
	"context"
	"strings"
	"time"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RunTestsKillStaleBundles,
		Desc:     "Verifies that Tast run kills already running local bundles",
		Attr:     []string{"group:mainline", "informational"},
		Contacts: []string{"oka@chromium.org", "tast-core@google.com"},
	})
}

func RunTestsKillStaleBundles(ctx context.Context, s *testing.State) {
	cmd, err := tastrun.NewCommand(ctx, s, "run", nil, []string{"meta.LocalFreezeForever"})
	if err != nil {
		s.Fatal("Creating first command to run tast: ", err)
	}
	r, err := cmd.StdoutPipe()
	if err != nil {
		s.Fatal("cmd.StdoutPipe(): ", err)
	}
	defer r.Close()

	s.Log("Starting ", strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		s.Fatal("Starting tast: ", err)
	}
	done := make(chan struct{})
	go func() {
		cmd.Wait()
		close(done)
	}()
	defer func() {
		// Concurrent Kill/Wait are handled gracefully by os/exec
		cmd.Process.Kill()
		<-done
	}()

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

	_, _, err = tastrun.Exec(ctx, s, "run", nil, []string{"meta.LocalPass"})
	if err != nil {
		s.Fatal("Second tast run failed: ", err)
	}

	shortCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	select {
	case <-done:
		// OK
	case <-shortCtx.Done():
		s.Fatal("The first command did not terminate")
	}
}
