// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// SupportdSetupContext contains the contexts to be used by the test
type SupportdSetupContext struct {
	// CleanupContext is used for teardown
	CleanupContext context.Context
	// CleanupContext is used for the actual test
	TestContext context.Context
}

// SetupSupportdForAPITest starts the Wilco DTC VM and Support Daemon
func SetupSupportdForAPITest(ctx context.Context, s *testing.State) (*SupportdSetupContext, error) {
	// Shorten the total context by 5 seconds to allow for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)

	// Expect the services to start within 5 seconds.
	startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	originalCtx := ctx

	ctx, st := timing.Start(ctx, "setup_supportd_for_api_test")
	defer st.End()

	config := wilco.DefaultVMConfig()
	config.StartProcesses = false
	if err := wilco.StartVM(startCtx, config); err != nil {
		return nil, errors.Wrap(err, "unable to Start Wilco DTC VM")
	}

	if err := wilco.StartSupportd(startCtx); err != nil {
		return nil, errors.Wrap(err, "unable to start Wilco DTC Support Daemon")
	}

	return &SupportdSetupContext{
		CleanupContext: cleanupCtx,
		TestContext:    originalCtx}, nil
}

// TeardownSupportdForAPITest stops the Wilco DTC VM and Support Daemon
func TeardownSupportdForAPITest(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "teardown_supportd_for_api_test")
	defer st.End()

	if err := wilco.StopVM(ctx); err != nil {
		s.Fatal("Unable to stop Wilco DTC VM: ", err)
	}

	if err := wilco.StopSupportd(ctx); err != nil {
		s.Fatal("Unable to stop Wilco DTC Support Daemon: ", err)
	}
}
