// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bluetooth contains test utilities for remote bluetooth tests.
package bluetooth

import (
	"context"
	"fmt"
	"time"

	pbmanager "go.chromium.org/chromiumos/config/go/test/api/test_libs/chameleond_manager"

	"chromiumos/tast/remote/chameleond/manager"
	"chromiumos/tast/testing"
)

// Timeout for methods of Tast fixture.
const (
	setUpTimeout    = 1 * time.Minute
	postTestTimeout = 10 * time.Second
	tearDownTimeout = 10 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "bluetoothChameleond",
		Desc: "Preliminary bluetooth testing fixture with a Chameleond client",
		Contacts: []string{
			"jaredbennett@google.com",
		},
		Impl:            &TestFixture{},
		Parent:          "chromeLoggedInWithBluetoothEnabled",
		SetUpTimeout:    setUpTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: tearDownTimeout,
		Vars:            []string{"chameleondCFTServiceAddr"},
	})
}

// TestFixture is an implementation of FixtureImpl for the bluetoothChameleond
// test fixture.
type TestFixture struct {
	ChameleondHosts []*pbmanager.ChameleondHost
	CMS             *manager.ChameleondManagerServiceClient
	vars            fixtureVars
}

type fixtureVars struct {
	ChameleondCFTServiceAddr string
}

// SetUp parses the test fixture variables and sets up the fixture with a
// connected ChameleondManagerServiceClient that is targeted at the first
// Chameleond host.
func (tf *TestFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	tf.vars = fixtureVars{
		ChameleondCFTServiceAddr: s.RequiredVar("chameleondCFTServiceAddr"),
	}
	var err error

	// Connect to CMS.
	tf.CMS, err = manager.NewChameleondManagerServiceClient(ctx, tf.vars.ChameleondCFTServiceAddr)
	if err != nil {
		s.Fatal("Failed to create new ChameleondManagerServiceClient: ", err)
	}

	// Collect available Chameleond hosts in the testbed.
	resp, err := tf.CMS.ChameleondManagerService.GetAvailableChameleondHosts(ctx, &pbmanager.GetAvailableChameleondHostsRequest{})
	if err != nil {
		s.Fatal("Failed to get available Chameleond hosts: ", err)
	}
	tf.ChameleondHosts = resp.Hosts
	testing.ContextLogf(ctx, "Found %d available Chameleond hosts", err)

	// Target first Chameleond host.
	if len(tf.ChameleondHosts) > 0 {
		target := tf.ChameleondHosts[0]
		hostStr := fmt.Sprintf("%s:%d'", target.Host, target.Port)
		testing.ContextLogf(ctx, "Targeting first Chameleond host %q", hostStr)
		_, err := tf.CMS.ChameleondManagerService.SetChameleondTarget(ctx, &pbmanager.SetChameleondTargetRequest{
			Target: target,
		})
		if err != nil {
			s.Fatalf("Failed to target Chameleond host %q: %v", hostStr, err)
		}
		testing.ContextLogf(ctx, "Successfully targeted Chameleond host %q", hostStr)
	} else {
		s.Fatal("Invalid test environment: this test bed has no Chameleond hosts")
	}

	return nil
}

// Reset does not do anything, but is necessary to include to implement the
// FixtureImpl interface.
func (tf *TestFixture) Reset(ctx context.Context) error {
	return nil
}

// PreTest does not do anything, but is necessary to include to implement the
// FixtureImpl interface.
func (tf *TestFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

// PostTest does not do anything yet, but is necessary to include to implement
// the FixtureImpl interface.
func (tf *TestFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// TODO(jaredbennett) collect cms logs
}

// TearDown cleans up fixture connections.
func (tf *TestFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := tf.CMS.Close(); err != nil {
		s.Fatal("Failed to close ChameleondManagerServiceClient: ", err)
	}
}
