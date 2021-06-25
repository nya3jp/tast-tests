// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/remote/wificell/wifiutil"
	"chromiumos/tast/testing"
)

// Timeout for methods of Tast fixture.
const (
	// Give long enough timeout for SetUp() and TearDown() as they might need
	// to reboot a broken DUT.
	setUpTimeout    = 5 * time.Minute
	tearDownTimeout = 5 * time.Minute
	resetTimeout    = 10 * time.Second
	postTestTimeout = 5 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "wificellFixt",
		Desc: "Default wificell setup",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Impl:            newTastFixture(TFFeaturesNone),
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: tearDownTimeout,
		ServiceDeps:     []string{TFServiceName},
		Vars:            []string{"router", "pcap"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "wificellFixtWithCapture",
		Desc: "Wificell setup with pcap for each configured AP",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Impl:            newTastFixture(TFFeaturesCapture),
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: tearDownTimeout,
		ServiceDeps:     []string{TFServiceName},
		Vars:            []string{"router", "pcap"},
	})
}

// tastFixtureImpl is the Tast implementation of the Wificell fixture.
// Notice the difference between tastFixtureImpl and TestFixture objects.
// The former is the one in the Tast framework; the latter is for
// wificell fixture.
type tastFixtureImpl struct {
	features TFFeatures
	tf       *TestFixture
}

// newTastFixture creates a Tast fixture with given features.
func newTastFixture(features TFFeatures) *tastFixtureImpl {
	return &tastFixtureImpl{
		features: features,
	}
}

// companionName returns the hostname of a companion device.
func (f *tastFixtureImpl) companionName(s *testing.FixtState, suffix string) string {
	name, err := s.DUT().CompanionDeviceHostname(suffix)
	if err != nil {
		s.Fatal("Unable to synthesize name, err: ", err)
	}
	return name
}

// recoverUnhealthyDUT checks if the DUT is healthy. If not, try to recover it
// with reboot.
func (f *tastFixtureImpl) recoverUnhealthyDUT(ctx context.Context, s *testing.FixtState) error {
	return recoverUnhealthyDUT(ctx, s.DUT(), s.RPCHint(), &f.tf)
}

func (f *tastFixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := f.recoverUnhealthyDUT(ctx, s); err != nil {
		s.Fatal("Failed to recover unhealthy DUT: ", err)
	}

	tf, err := setUpTestFixture(ctx, s.FixtContext(), s.DUT(), s.RPCHint(), f.features, s.Var)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	f.tf = tf

	return f.tf
}

func (f *tastFixtureImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	// Ensure DUT is healthy here again, so that we don't leave with
	// bad state to later tests/tasks.
	if err := f.recoverUnhealthyDUT(ctx, s); err != nil {
		s.Fatal("Failed to recover unhealthy DUT: ", err)
	}

	if f.tf == nil {
		return
	}
	if err := f.tf.Close(ctx); err != nil {
		s.Log("Failed to tear down test fixture, err: ", err)
	}
	f.tf = nil
}

func (f *tastFixtureImpl) Reset(ctx context.Context) error {
	var firstErr error
	// Light-weight health check here. SetUp/TearDown will try to recover
	// the DUT when anything goes wrong.
	if _, err := f.tf.WifiClient().HealthCheck(ctx, &empty.Empty{}); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, err)
	}
	if err := f.tf.Reinit(ctx); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, err)
	}
	return firstErr
}

func (f *tastFixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// Nothing to do here for now.
}

func (f *tastFixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if err := f.tf.CollectLogs(ctx); err != nil {
		s.Log("Error collecting logs, err: ", err)
	}
}
