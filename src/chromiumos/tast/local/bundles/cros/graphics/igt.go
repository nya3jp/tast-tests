// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func uiStopped() testing.Precondition { return uiStoppedPre }

var uiStoppedPre = &preImpl{
	name:    "ui_stopped",
	timeout: 30 * time.Second,
}

type preImpl struct {
	name    string
	timeout time.Duration
}

// Interface methods for a testing.Precondition.
func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	// This is a no-op if the job is not currently running.
	err := upstart.StopJob(ctx, "ui")
	if err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	return nil
}

func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Log("Failed to restart ui job: ", err)
	}
}

// igtTest is used to describe the config used to run each test.
type igtTest struct {
	exe     string        // The test executable name.
	timeout time.Duration // Timeout to run the test.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: IGT,
		Desc: "Verifies igt-gpu-tools test binaries run successfully",
		Contacts: []string{
			"ddavenport@chromium.org",
			"chromeos-gfx@google.com",
		},
		Params: []testing.Param{{
			Name: "kms_atomic",
			Val: igtTest{
				exe:     "kms_atomic",
				timeout: 5 * time.Minute,
			},
			ExtraSoftwareDeps: []string{"drm_atomic"},
			ExtraAttr:         []string{"informational"},
			Pre:               uiStopped(),
		}},
		Timeout: 5 * time.Minute,
		Attr:    []string{"group:mainline"},
	})
}

func IGT(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(igtTest)

	f, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(testOpt.exe)+".txt"))
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, testOpt.timeout)
	defer cancel()

	exePath := filepath.Join("/usr/local/libexec/igt-gpu-tools", testOpt.exe)
	cmd := testexec.CommandContext(ctx, exePath)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(); err != nil {
		s.Errorf("Failed to run %s: %v", exePath, err)
	}
}
