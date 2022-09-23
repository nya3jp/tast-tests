// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/upstart"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            fixture.ServicesOnBoot,
		Desc:            "Fixture for platform.ServicesOnBoot.* tests; DO NOT USE for other tests",
		Contacts:        []string{"aaronyu@google.com", "chromeos-audio-sw@google.com"},
		Impl:            ServicesOnBootFixt{},
		SetUpTimeout:    5 * time.Minute,
		TearDownTimeout: 1 * time.Minute,
		ServiceDeps:     []string{"tast.cros.platform.UpstartService"},
	})
}

// ServicesOnBootFixt is a fixture for the platform.ServiceOnBoot.* tests.
// DO NOT USE it on other tests.
type ServicesOnBootFixt struct{}

var _ testing.FixtureImpl = ServicesOnBootFixt{}

func (ServicesOnBootFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Perform a reboot so that events happen before the test starts do not
	// have an impact on the test result. For example other tests failing a
	// service should not fail the platform.ServiceOnBoot.* tests.
	s.Log("Rebooting DUT")
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// Leave time for clean up.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, time.Minute)
	defer cancel()

	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}

	defer func(ctx context.Context) {
		if err := d.Close(ctx); err != nil {
			s.Fatal("Failed to close RPCDUT: ", err)
		}
	}(cleanupCtx)

	s.Log("Waiting for system-services")
	upstartService := platform.NewUpstartServiceClient(d.RPC().Conn)
	if _, err := upstartService.WaitForJobStatus(ctx, &platform.WaitForJobStatusRequest{
		JobName: "system-services",
		Goal:    string(upstart.StartGoal),
		State:   string(upstart.RunningState),
		Timeout: durationpb.New(3 * time.Minute),
	}); err != nil {
		s.Fatal("Failed to wait for system-services: ", err)
	}

	// Sleep to observe service failures.
	// Generally, we recommend polling with a timeout instead of sleeping,
	// due to sleeping makes the test either slow or flaky.
	// However here, we're waiting for a service to fail. We don't have a
	// good way to tell if services have finished their initialization
	// sequences and can no longer fail spontaneously.
	// For our case, the happy path we should reach the timeout.
	// Polling with a timeout instead of sleeping makes only the sad path faster if we
	// detect failures early, but optimizing the sad path is not useful.
	const sleepDuration = 30 * time.Second
	s.Logf("Sleeping for %s to collect service activity", sleepDuration)
	testing.Sleep(ctx, sleepDuration)

	return nil
}

func (ServicesOnBootFixt) Reset(ctx context.Context) error {
	return nil
}

func (ServicesOnBootFixt) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (ServicesOnBootFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (ServicesOnBootFixt) TearDown(ctx context.Context, s *testing.FixtState) {}
