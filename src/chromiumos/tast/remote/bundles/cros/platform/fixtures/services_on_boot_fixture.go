// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            ServicesOnBootFixt,
		Desc:            "Reboot and collect upstart service failures",
		Contacts:        []string{"aaronyu@google.com", "chromeos-audio-sw@google.com"},
		Impl:            &servicesOnBootFixt{},
		SetUpTimeout:    5 * time.Minute,
		TearDownTimeout: 1 * time.Minute,
		ServiceDeps:     []string{"tast.cros.platform.UpstartService"},
	})
}

// servicesOnBootFixt is a fixture for the platform.ServiceOnBoot test.
type servicesOnBootFixt struct {
	ServicesOnBootFixtVal
	rpcdut *rpcdut.RPCDUT
}

// ServicesOnBootFixtVal contains public fields for the platform.ServiceOnBoot test.
type ServicesOnBootFixtVal struct {
	Failures []ServiceFailure
	Upstart  platform.UpstartServiceClient
}

var _ testing.FixtureImpl = &servicesOnBootFixt{}

// ServiceFailure is a log entry from upstart logs.
type ServiceFailure struct {
	Message        string
	JobName        string
	ProcessName    string
	ExitStatus     int // The exit status of the job. -1 means abnormal exit.
	KilledBySignal string
}

func (fixt *servicesOnBootFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	s.Log("Rebooting DUT")
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	var err error
	fixt.rpcdut, err = rpcdut.NewRPCDUT(s.FixtContext(), s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}

	fixt.Upstart = platform.NewUpstartServiceClient(fixt.rpcdut.RPC().Conn)
	s.Log("Waiting for system-services")
	if _, err := fixt.Upstart.WaitForJobStatus(ctx, &platform.WaitForJobStatusRequest{
		JobName: "system-services",
		Goal:    string(upstart.StartGoal),
		State:   string(upstart.RunningState),
		Timeout: durationpb.New(3 * time.Minute),
	}); err != nil {
		s.Fatal("Failed to wait for system-services: ", err)
	}

	const sleepDuration = 30 * time.Second
	s.Logf("Sleeping for %s to collect service activity", sleepDuration)
	testing.Sleep(ctx, sleepDuration)

	// Parse dmesg to get service failures
	// Not using /var/log/upstart.log as we're not interested in service
	// failures from previous boots.
	dmesg, err := s.DUT().Conn().CommandContext(ctx, "dmesg").Output()
	if err != nil {
		s.Fatal("Failed to run dmesg: ", err)
	}
	fixt.Failures, err = parseServiceFailures(string(dmesg))
	if err != nil {
		s.Fatal("Cannot parse service failures from demsg: ", err)
	}

	return fixt
}

func parseServiceFailures(dmesg string) ([]ServiceFailure, error) {
	initRE := regexp.MustCompile(`\[[0-9\. ]+\] init: (\S+) (\S+) process \(\d+\) (?:terminated with status (\d+)|killed by (\S+) signal)`)
	messages := initRE.FindAllStringSubmatch(string(dmesg), -1)
	result := make([]ServiceFailure, 0, len(messages))
	for _, message := range messages {
		failure := ServiceFailure{
			Message:     message[0],
			JobName:     message[1],
			ProcessName: message[2],
		}
		if message[3] != "" {
			var err error
			failure.ExitStatus, err = strconv.Atoi(message[3])
			if err != nil {
				return nil, errors.Errorf("cannot parse exit status of %q", message[0])
			}
		} else {
			failure.ExitStatus = -1
			failure.KilledBySignal = message[4]
		}
		result = append(result, failure)
	}
	return result, nil
}

func (*servicesOnBootFixt) Reset(ctx context.Context) error {
	return nil
}

func (*servicesOnBootFixt) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (*servicesOnBootFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (fixt *servicesOnBootFixt) TearDown(ctx context.Context, s *testing.FixtState) {
	if fixt.rpcdut != nil {
		if err := fixt.rpcdut.Close(ctx); err != nil {
			s.Fatal("Failed to close RPCDUT: ", err)
		}
	}

	fixt = &servicesOnBootFixt{}
}
