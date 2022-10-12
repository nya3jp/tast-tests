// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

const (
	fixtureSetUpTimeout    = 10 * time.Second
	fixtureResetTimeout    = 5 * time.Second
	fixtureTearDownTimeout = 5 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "ussAuthSessionFixture",
		Desc: "Set up the USS flag experiement flag for Auth Session",
		Contacts: []string{
			"lziest@google.com",
			"cryptohome-core@google.com",
		},
		SetUpTimeout:    fixtureSetUpTimeout,
		ResetTimeout:    fixtureResetTimeout,
		TearDownTimeout: fixtureTearDownTimeout,
		Impl: &fixtureImpl{
			ussFlag: true,
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "vkAuthSessionFixture",
		Desc: "Disable the USS flag experiement flag for Auth Session",
		Contacts: []string{
			"lziest@google.com",
			"cryptohome-core@google.com",
		},
		SetUpTimeout:    fixtureSetUpTimeout,
		ResetTimeout:    fixtureResetTimeout,
		TearDownTimeout: fixtureTearDownTimeout,
		Impl: &fixtureImpl{
			ussFlag: false,
		},
	})
}

type cleanupFunc func(context.Context) error

type fixtureImpl struct {
	ussFlag        bool
	ussFlagCleanup cleanupFunc
}

// AuthSessionFixture provides data on how the session has been configured by the fixture.
type AuthSessionFixture struct {
	UssEnabled bool
}

func (f *fixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	// Wait for cryptohomed becomes available.
	daemonController := helper.DaemonController()
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}
	if err := UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount all: ", err)
	}

	if f.ussFlag {
		// Enable the UserSecretStash experiment for the duration of the test by
		// creating a flag file that's checked by cryptohomed.
		// A cleanup routine is returned by the helper function. We will run it
		// when tearing down the test environment.
		var err error
		f.ussFlagCleanup, err = helper.EnableUserSecretStash(ctx)
		if err != nil {
			s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
		}
	} else {
		// Disable the UserSecretStash experiment for the duration of the test
		// ensuring that the flag file that enables it does not exist.
		//
		// This mode has no cleanup as this is the "default" state.
		err := helper.DisableUserSecretStash(ctx)
		if err != nil {
			s.Error("Failed to clean up the USS flag during setup: ", err)
		}
	}
	return &AuthSessionFixture{
		UssEnabled: f.ussFlag,
	}
}

func (f *fixtureImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := UnmountAll(ctx); err != nil {
		s.Error("Failed to unmount all: ", err)
	}
	if f.ussFlag {
		err := f.ussFlagCleanup(ctx)
		if err != nil {
			s.Error("Failed to clean up the USS flag: ", err)
		}
	}
}

func (f *fixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *fixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *fixtureImpl) Reset(ctx context.Context) error {
	// Clean up obsolete state, in case there's any.
	if err := UnmountAll(ctx); err != nil {
		return err
	}
	return nil
}
