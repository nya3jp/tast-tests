// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/shillscript"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShillInitLogoutScript,
		Desc:         "Test that shill init logout script perform as expected",
		Contacts:     []string{"arowa@google.com", "cros-networking@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func ShillInitLogoutScript(ctx context.Context, s *testing.State) {
	if err := shillscript.RunTest(ctx, testLogout); err != nil {
		s.Fatal("Failed running testLogout: ", err)
	}
}

// testLogout tests the logout process.
func testLogout(ctx context.Context, env *shillscript.TestEnv) error {
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, shillscript.DbusMonitorTimeout)
	defer cancel()

	stop, err := shillscript.DbusEventMonitor(timeoutCtx)
	if err != nil {
		return err
	}

	// Emulate logout.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		_, _ = stop()
		return errors.Wrap(err, "Chrome logout failed")
	}

	calledMethods, err := stop()
	if err != nil {
		return err
	}

	expectedCalls := []string{shillscript.PopAllUserProfiles}
	if err := shillscript.AssureMethodCalls(ctx, expectedCalls, calledMethods); err != nil {
		return err
	}

	if err := shillscript.AssureNotExists(shillscript.ShillUserProfilesDir); err != nil {
		return err
	}

	profiles, err := shillscript.GetProfileList(ctx)
	if err != nil {
		return err
	}

	if len(profiles) > 1 {
		return errors.Wrapf(err, "found unexpected number of profiles in the profile stack: got %d, want 1", len(profiles))
	}

	return nil
}
