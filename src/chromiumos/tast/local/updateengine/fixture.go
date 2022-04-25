// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateengine

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Pseudo-timeouts.
const (
	updateEngineResetTimeout    = 20 * time.Second
	updateEngineSetUpTimeout    = 20 * time.Second
	updateEngineTearDownTimeout = 20 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "updateEngineReady",
		Desc:            "Update Engine is running with a clean state",
		Contacts:        []string{"chromeos-core-services@google.com"},
		Impl:            &updateEngineFixture{},
		ResetTimeout:    updateEngineResetTimeout,
		SetUpTimeout:    updateEngineSetUpTimeout,
		TearDownTimeout: updateEngineTearDownTimeout,
	})
}

type updateEngineFixture struct {
}

func (*updateEngineFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	testing.ContextLog(ctx, "SetUp update_engine fixture")
	if err := StartDaemon(ctx); err != nil {
		s.Fatal("Failed to start update_engine: ", err)
	}
	return nil
}

// Reset must clear update_engine states back to a fresh one.
func (*updateEngineFixture) Reset(ctx context.Context) error {
	testing.ContextLog(ctx, "Reset update_engine fixture")
	if err := ClearPrefs(ctx); err != nil {
		return errors.Wrap(err, "failed to reset update_engine state")
	}
	return nil
}

func (*updateEngineFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (*updateEngineFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (*updateEngineFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	testing.ContextLog(ctx, "TearDown update_engine fixture")
	if err := ClearOobeCompletion(ctx); err != nil {
		s.Error("Failed to clear OOBE completed flag during teardown: ", err)
	}
	if err := StopDaemon(ctx); err != nil {
		s.Error("Failed to stop update_engine during teardown: ", err)
	}
	if err := ForceClearPrefs(ctx); err != nil {
		s.Error("Failed to force clear prefs during teardown: ", err)
	}
	// Failure to bring update_engine daemon up is fatal.
	if err := StartDaemon(ctx); err != nil {
		s.Fatal("Failed to start update_engine during teardown: ", err)
	}
	if err := WaitForService(ctx); err != nil {
		s.Error("Failed to wait for update_engine service during teardown: ", err)
	}
}
