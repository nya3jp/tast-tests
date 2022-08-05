// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	btc "chromiumos/tast/common/bluetooth"
	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	bts "chromiumos/tast/services/cros/bluetooth"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// fixtureVarBTPeers is the name of the tast var that specifies a
// comma-separated list of btpeer host addresses.
//
// This is an optional override to the usual btpeer addresses which are normally
// resolved based on the DUT hostname.
const fixtureVarBTPeers = "btpeers"

var fixtureVars = []string{
	fixtureVarBTPeers,
}

// maxAvailableBTPeers is the maximum number of btpeers that are expected to
// be available for a test.
const maxAvailableBTPeers = 4

func init() {
	addFixture("chromeLoggedInWithBluetoothRevampEnabled",
		"Logs into a user session with the BluetoothRevamp feature flag enabled",
		&fixtureFeatures{
			BluetoothRevamp: true,
		})

	addFixture("chromeLoggedInWithBluetoothRevampDisabled",
		"Logs into a user session with the BluetoothRevamp feature flag disabled",
		&fixtureFeatures{
			BluetoothRevamp: false,
		})

	addFixture("chromeLoggedInWithBluetoothEnabled",
		"Logs into a user session and enables Bluetooth during set up and disables it during tear down",
		&fixtureFeatures{
			BluetoothRevamp:         true,
			BluetoothAdapterEnabled: true,
		})

	// Add a test fixture for different btpeer counts.
	for i := 1; i <= maxAvailableBTPeers; i++ {
		addFixture(fmt.Sprintf("chromeLoggedInWith%dBTPeers", i),
			fmt.Sprintf("Logs into a user session, enables Bluetooth, and connects to %d btpeers", i),
			&fixtureFeatures{
				BluetoothRevamp:         true,
				BluetoothAdapterEnabled: true,
				BTPeerCount:             i,
			})
	}
}

func addFixture(name, desc string, features *fixtureFeatures) {
	btpeerResetTimeoutBuffer := time.Duration(features.BTPeerCount*15) * time.Second
	testing.AddFixture(&testing.Fixture{
		Name: name,
		Desc: desc,
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            newFixture(features),
		Vars:            fixtureVars,
		SetUpTimeout:    20*time.Second + btpeerResetTimeoutBuffer,
		ResetTimeout:    5*time.Second + btpeerResetTimeoutBuffer,
		TearDownTimeout: 10*time.Second + btpeerResetTimeoutBuffer,
		ServiceDeps:     []string{"tast.cros.bluetooth.BTTestService"},
	})
}

type fixtureFeatures struct {
	// BTPeerCount requires the specified amount of btpeers to exist in the
	// testbed and connects to them during setup. A testbed can have more btpeers
	// than the BTPeerCount, but only that many connections are configured.
	BTPeerCount int

	// BluetoothAdapterEnabled being true will cause the DUT bluetooth adapter
	// to be enabled during fixture.SetUp and fixture.Reset and disabled during
	// fixture.TearDown.
	BluetoothAdapterEnabled bool

	// BluetoothRevamp is used as the value for ChromeNewRequest.BluetoothRevamp
	// when BTTestService.ChromeNew is called during fixture.SetUp to toggle
	// the "BluetoothRevamp" feature during Chrome login. A true value will
	// enable the feature and a false value will disable the feature.
	BluetoothRevamp bool
}

// FixtValue is the value of the test fixture accessible within a test. All
// variables are configured in fixture.SetUp so that tests can use them without
// any further configuration.
type FixtValue struct {
	// BTPeers is a list of chameleond clients that are connected to each btpeer
	// available to the test fixture.
	BTPeers []chameleon.Chameleond

	// DUTRPCClient is a gRPC client that remains connected to the DUT throughout
	// the life of the test fixture. This can be used to create clients to
	// additional local tast services.
	DUTRPCClient *rpc.Client

	// BTS is a client of the BTTestService that uses the DUTRPCClient connection.
	BTS bts.BTTestServiceClient
}

type fixture struct {
	features *fixtureFeatures
	fv       *FixtValue
}

func newFixture(features *fixtureFeatures) *fixture {
	return &fixture{
		features: features,
		fv: &FixtValue{
			BTPeers: make([]chameleon.Chameleond, 0),
		},
	}
}

// SetUp preforms fixture setup actions. All fixtureFeatures are configured.
//
// This is necessary to implement testing.FixtureImpl.
func (tf *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Connect to btpeers and reset them to a fresh state.
	if tf.features.BTPeerCount > 0 {
		if err := tf.setUpBTPeers(ctx, s, tf.features.BTPeerCount); err != nil {
			s.Fatal("Failed to set up BTPeers: ", err)
		}
		if err := tf.resetBTPeers(ctx); err != nil {
			s.Fatal("Failed to reset all btpeers: ", err)
		}
	}

	// Connect to local gRPC services, and keep connection alive until after
	// TearDown is called by using the fixture context.
	rpcClient, err := rpc.Dial(s.FixtContext(), s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the local gRPC service on the DUT: ", err)
	}
	tf.fv.DUTRPCClient = rpcClient
	tf.fv.BTS = bts.NewBTTestServiceClient(tf.fv.DUTRPCClient.Conn)

	// Log into chrome.
	_, err = tf.fv.BTS.ChromeNew(ctx, &bts.ChromeNewRequest{
		BluetoothRevampEnabled: tf.features.BluetoothRevamp,
	})
	if err != nil {
		s.Fatal("Failed to log into chrome on DUT: ", err)
	}

	// Enable bluetooth adapter.
	if tf.features.BluetoothAdapterEnabled {
		_, err = tf.fv.BTS.EnableBluetoothAdapter(ctx, &emptypb.Empty{})
		if err != nil {
			s.Fatal("Failed to enable bluetooth adapter on DUT: ", err)
		}
	}
	return tf.fv
}

// Reset is called by the framework after each test (except for the last one) to
// do a light-weight reset of the environment to the original state.
//
// This is necessary to implement testing.FixtureImpl.
func (tf *fixture) Reset(ctx context.Context) (retErr error) {
	if tf.features.BluetoothAdapterEnabled {
		_, err := tf.fv.BTS.EnableBluetoothAdapter(ctx, &emptypb.Empty{})
		if err != nil {
			return errors.Wrap(err, "failed to enable bluetooth adapter on DUT")
		}
	}
	if err := tf.resetBTPeers(ctx); err != nil {
		return errors.Wrap(err, "failed to reset all btpeers")
	}
	return nil
}

// PreTest is called by the framework before each test to do a light-weight set
// up for the test.
//
// This is necessary to implement testing.FixtureImpl.
func (tf *fixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// No-op.
}

// PostTest is called by the framework after each test to tear down changes
// PreTest made.
//
// This is necessary to implement testing.FixtureImpl.
func (tf *fixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// No-op.
}

// TearDown is called by the framework to tear down the environment SetUp set
// up.
//
// This is necessary to implement testing.FixtureImpl.
func (tf *fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Turn bluetooth adapter back off.
	if tf.features.BluetoothAdapterEnabled {
		if _, err := tf.fv.BTS.DisableBluetoothAdapter(ctx, &emptypb.Empty{}); err != nil {
			s.Fatal("Failed to disable bluetooth adapter on DUT: ", err)
		}
	}

	// Clean up chrome login state.
	if _, err := tf.fv.BTS.ChromeClose(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to call ChromeClose: ", err)
	}

	// Close gRPC connection to DUT.
	if err := tf.fv.DUTRPCClient.Close(ctx); err != nil {
		s.Fatal("Failed to close gRPC connection to DUT: ", err)
	}

	// Return btpeers to original state.
	if err := tf.resetBTPeers(ctx); err != nil {
		s.Fatal("Failed to reset all btpeers: ", err)
	}
}

func (tf *fixture) setUpBTPeers(ctx context.Context, s *testing.FixtState, requiredBTPeers int) error {
	ctx, st := timing.Start(ctx, fmt.Sprintf("setUpBTPeers_%d", requiredBTPeers))
	defer st.End()
	if requiredBTPeers <= 0 {
		return nil
	}
	var btpeerAddresses []string
	if btpeersVar, isSet := s.Var(fixtureVarBTPeers); isSet && btpeersVar != "" {
		btpeerAddresses = strings.Split(btpeersVar, ",")
		if len(btpeerAddresses) < requiredBTPeers {
			return errors.Errorf("fixture requires at least %d btpeers, but "+
				"only %d were provided in the %s tast var (%q)",
				requiredBTPeers, len(btpeerAddresses),
				fixtureVarBTPeers, btpeersVar)
		}
		btpeerAddresses = btpeerAddresses[:requiredBTPeers]
	} else {
		// Imply btpeer hostnames based on DUT hostname.
		btpeerAddresses = make([]string, requiredBTPeers)
		dutHostname := strings.Split(s.DUT().HostName(), ":")[0]
		if dutHostname == "localhost" {
			for i := 0; i < requiredBTPeers; i++ {
				btpeerAddresses[i] = fmt.Sprintf("localhost:%d", 2201+i)
			}
			exampleTastCall := fmt.Sprintf("tast run --var=%s=%s %s <test>",
				fixtureVarBTPeers, strings.Join(btpeerAddresses, ","),
				s.DUT().HostName())
			return errors.Errorf("btpeer hostname resolution not supported "+
				"when DUT hostname is \"localhost\". If tast is being run in a local "+
				"development environment outside of the lab, ssh tunnel to the "+
				"btpeers outside of the chroot and provide the local forwarded"+
				" address to tast using the %q tast var (e.g. %q)",
				fixtureVarBTPeers, exampleTastCall)
		}
		for i := 0; i < requiredBTPeers; i++ {
			btpeerAddresses[i] = fmt.Sprintf("%s-btpeer%d", dutHostname, i+1)
		}
	}
	testing.ContextLogf(ctx, "Connecting to %d btpeers: %s",
		len(btpeerAddresses), strings.Join(btpeerAddresses, ", "))
	btpeers, err := btc.ConnectToBTPeers(ctx, btpeerAddresses)
	if err != nil {
		return err
	}
	tf.fv.BTPeers = btpeers
	testing.ContextLogf(ctx, "Successfully connected to %d btpeers",
		len(tf.fv.BTPeers))
	return nil
}

// resetBTPeers resets each configured btpeer to return them to their normal
// state and clear any changes a test may have made to them.
func (tf *fixture) resetBTPeers(ctx context.Context) error {
	ctx, st := timing.Start(ctx, fmt.Sprintf("resetBTPeers_%d", len(tf.fv.BTPeers)))
	defer st.End()
	for i, btpeer := range tf.fv.BTPeers {
		if err := btpeer.Reset(ctx); err != nil {
			return errors.Wrapf(err, "failed to reset btpeer[%d] at %q", i,
				btpeer.Host())
		}
	}
	return nil
}
