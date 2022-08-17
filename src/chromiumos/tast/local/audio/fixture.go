// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "crasStopped",
		Desc:            "Ensure CRAS is stopped and audio devices are available",
		Contacts:        []string{"aaronyu@google.com"},
		Impl:            crasStoppedFixture{},
		SetUpTimeout:    20 * time.Second,
		TearDownTimeout: 20 * time.Second,
	})
}

// crasStoppedFixture is a fixture to stop CRAS and sleep for a while to
// ensure that (ALSA) audio devices are available.
// Tests that need to access audio devices directly should use this fixture.
type crasStoppedFixture struct{}

func (crasStoppedFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := upstart.StopJob(ctx, "cras"); err != nil {
		s.Fatal("Cannot stop cras: ", err)
	}

	// We might need to sleep longer to work around improperly powered down devices.
	// See b/232799132 for an example that generated flakes.
	const sleepDuration = 1 * time.Second

	s.Logf("Sleeping for %s to wait for audio device to be ready", sleepDuration)
	if err := testing.Sleep(ctx, sleepDuration); err != nil {
		s.Fatal("Sleep failed: ", err)
	}
	return nil
}

func (crasStoppedFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := RestartCras(ctx); err != nil {
		s.Fatal("Cannot restart cras: ", err)
	}
}

func (crasStoppedFixture) Reset(ctx context.Context) error {
	return nil
}

func (crasStoppedFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (crasStoppedFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}
