// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/shillscript"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShillInitScriptsLoginProfileExists,
		Desc:         "Test that shill init scripts perform as expected",
		Contacts:     []string{"arowa@google.com", "cros-networking@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func ShillInitScriptsLoginProfileExists(ctx context.Context, s *testing.State) {
	if err := shillscript.RunTest(ctx, testLoginProfileExists, false); err != nil {
		s.Fatal("Failed running testLoginProfileExists: ", err)
	}
}

// testLoginProfileExists tests logging in a user whose profile already exists.
// Login script should only push (and not create) the user profile
// if a user profile already exists.
func testLoginProfileExists(ctx context.Context, env *shillscript.TestEnv) error {
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return errors.Wrap(err, "Chrome failed to log in")
	}
	cr.Close(ctx)

	cr2, err := chrome.New(ctx, chrome.DeferLogin(), chrome.KeepState())
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	defer cr2.Close(ctx)

	timeoutCtx, cancel := context.WithTimeout(ctx, shillscript.DbusMonitorTimeout)
	defer cancel()

	stop, err := shillscript.DbusEventMonitor(timeoutCtx)
	if err != nil {
		return err
	}

	if err := cr2.ContinueLogin(ctx); err != nil {
		stop()
		return errors.Wrap(err, "Chrome failed to log in")
	}

	calledMethods, err := stop()
	if err != nil {
		return err
	}

	expectedCalls := []string{shillscript.InsertUserProfile}
	if err := shillscript.AssureMethodCalls(ctx, expectedCalls, calledMethods); err != nil {
		return err
	}

	profiles, err := shillscript.GetProfileList(ctx)
	if err != nil {
		return err
	}

	if len(profiles) != 2 {
		return errors.Wrapf(err, "found unexpected number of profiles in the profile stack: got %d, want 2", len(profiles))
	}

	return nil
}
