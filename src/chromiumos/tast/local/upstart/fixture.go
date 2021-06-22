// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package upstart interacts with the Upstart init daemon on behalf of local tests.
package upstart

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// UIRestartTimeout is the maximum amount of time that it takes to restart
// the ui upstart job.
// ui-post-stop can sometimes block for an extended period of time
// waiting for "cryptohome --action=pkcs11_terminate" to finish: https://crbug.com/860519
const UIRestartTimeout = 60 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "ensureUI",
		Desc: "Ensure the ui service is running",
		Contacts: []string{
			"pwang@chromium.org", // fixture author
			"cros-printing-dev@chromium.org",
		},
		Impl:            &ensureUIFixture{running: true},
		SetUpTimeout:    10 * time.Second,
		TearDownTimeout: 10 * time.Second,
		ResetTimeout:    10 * time.Second,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "ensureNoUI",
		Desc: "Ensure the ui service is not running",
		Contacts: []string{
			"tbegin@chromium.org", // fixture author
			"cros-network-health@google.com",
		},
		Impl:            &ensureUIFixture{running: false},
		SetUpTimeout:    UIRestartTimeout,
		TearDownTimeout: UIRestartTimeout,
		ResetTimeout:    1 * time.Second,
	})
}

type ensureUIFixture struct {
	running bool
}

func (f *ensureUIFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if f.running {
		if err := EnsureJobRunning(ctx, "ui"); err != nil {
			s.Fatal("Failed to start ui: ", err)
		}
	} else {
		if err := StopJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to stop ui: ", err)
		}
	}

	return nil
}

func (f *ensureUIFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// We intentionally don't stop the ui as Chrome running is the expected
	// clean. If the ui is stopped, attempt to start it.
	if err := EnsureJobRunning(ctx, "ui"); err != nil {
		s.Log("Failed to start ui: ", err)
	}
}

func (f *ensureUIFixture) Reset(ctx context.Context) error {
	// Tried to ensure the ui job is still running between tests. EnsureJobRunning is noop if Chrome is running.
	if f.running {
		if err := EnsureJobRunning(ctx, "ui"); err != nil {
			return errors.Wrap(err, "failed to ensure ui is running")
		}
	}
	return nil
}

func (f *ensureUIFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *ensureUIFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}
