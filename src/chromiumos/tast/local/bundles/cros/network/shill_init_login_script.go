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
		Func:     ShillInitLoginScript,
		Desc:     "Test that shill init login script perform as expected",
		Contacts: []string{"arowa@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func ShillInitLoginScript(ctx context.Context, s *testing.State) {
	if err := shillscript.RunTest(ctx, testLogin); err != nil {
		s.Fatal("Failed running testLogin: ", err)
	}
}

// testLogin tests the login process.
// Login should create a profile directory, then create and push
// a user profile, given no previous state.
func testLogin(ctx context.Context, env *shillscript.TestEnv) error {
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, shillscript.DbusMonitorTimeout)
	defer cancel()

	stop, err := shillscript.DbusEventMonitor(timeoutCtx)
	if err != nil {
		return err
	}

	if err := shillscript.Login(ctx, shillscript.FakeUser); err != nil {
		_, _ = stop()
		return errors.Wrap(err, "failed logging in")
	}

	calledMethods, err := stop()
	if err != nil {
		return err
	}

	expectedCalls := []string{shillscript.CreateUserProfile, shillscript.InsertUserProfile}
	if err := shillscript.AssureMethodCalls(ctx, expectedCalls, calledMethods); err != nil {
		return err
	}

	if err := shillscript.AssureExists(env.ShillUserProfile); err != nil {
		return errors.Wrapf(err, "shill user profile %s doesn't exist", env.ShillUserProfile)
	}

	if err := shillscript.AssureIsDir(env.ShillUserProfileDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %v is a directory", env.ShillUserProfileDir)
	}

	if err := shillscript.AssureIsDir(shillscript.ShillUserProfilesDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %s is a directory", shillscript.ShillUserProfilesDir)
	}

	if err := shillscript.AssureIsLinkTo(shillscript.ShillUserProfileChronosDir, env.ShillUserProfileDir); err != nil {
		return err
	}

	if err := shillscript.AssureIsDir(env.UserCryptohomeLogDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %v is a directory", env.UserCryptohomeLogDir)
	}

	if err := shillscript.AssureIsLinkTo("/run/shill/log", env.UserCryptohomeLogDir); err != nil {
		return err
	}

	profiles, err := shillscript.GetProfileList(ctx)
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		return errors.Wrap(err, "profile list is empty")
	}

	// The last profile should be the one we just created.
	profilePath := profiles[len(profiles)-1].String()

	if profilePath != shillscript.ExpectedProfileName {
		return errors.Wrapf(err, "found unexpected profile path: got %s, want %s", profilePath, shillscript.ExpectedProfileName)
	}

	return nil
}
