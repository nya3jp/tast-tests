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

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "ensureUI",
		Desc:            "Ensure the ui service is running.",
		Impl:            &ensureUIFixture{},
		SetUpTimeout:    10 * time.Second,
		TearDownTimeout: 10 * time.Second,
		ResetTimeout:    10 * time.Second,
	})
}

type ensureUIFixture struct {
}

func (f *ensureUIFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := EnsureJobRunning(ctx, "ui"); err != nil {
		s.Fatal("Failed to start ui: ", err)
	}
	return nil
}

func (f *ensureUIFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// We intentionally not stopping the ui as most of the tests is requiring the ui.
}

func (f *ensureUIFixture) Reset(ctx context.Context) error {
	// Tried to ensure the ui job is still running between test. EnsureJobRunning should be noop if the ui is already running.
	if err := EnsureJobRunning(ctx, "ui"); err != nil {
		return errors.Wrap(err, "failed to ensure ui is running")
	}
	return nil
}

func (f *ensureUIFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *ensureUIFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}
