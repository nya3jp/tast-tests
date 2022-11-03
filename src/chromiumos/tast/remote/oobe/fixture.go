// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/rpc"
	chromeService "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

const fixtureVarSigninKey = "ui.signinProfileTestExtensionManifestKey"

const (
	defaultUsername = "testuser@gmail.com"
	defaultPassword = "testpass"
)

const (
	serviceDepChromeService = "tast.cros.browser.ChromeService"
)

const (
	setUpTimeout    = 20 * time.Second
	resetTimeout    = 5 * time.Second
	tearDownTimeout = 10 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "chromeOobeHidDetection",
		Desc: "Puts the DUT into OOBE HID Detection Screen",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl: newFixture(&fixtureFeatures{
			EnableFeatures:        []string{"OobeHidDetectionRevamp"},
			DisableFeatures:       []string{},
			LoginMode:             chromeService.LoginMode_LOGIN_MODE_NO_LOGIN,
			EnableHidScreenOnOobe: true,
		}),
		Vars:         []string{fixtureVarSigninKey},
		SetUpTimeout: setUpTimeout,
		ResetTimeout: resetTimeout,
		ServiceDeps:  []string{serviceDepChromeService},
	})
}

type fixtureFeatures struct {
	// EnableFeatures is the list of features that will be enabled when starting Chrome.
	EnableFeatures []string

	// DisableFeatures is the list of features that will be enabled when starting Chrome.
	DisableFeatures []string

	// LoginMode is what the resulting login mode should be after starting Chrome.
	LoginMode chromeService.LoginMode

	// EnableHidScreenOnOobe enables HID detection screen when in OOBE.
	EnableHidScreenOnOobe bool
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

	// ChromeService is a client of the ChromeService that is used to start Chrome.
	ChromeService chromeService.ChromeServiceClient
}

type fixture struct {
	features *fixtureFeatures
	fv       *FixtValue
}

func newFixture(features *fixtureFeatures) *fixture {
	return &fixture{
		features: features,
		fv:       &FixtValue{},
	}
}

// SetUp preforms fixture setup actions. All fixtureFeatures are configured.
//
// This is necessary to implement testing.FixtureImpl.
func (tf *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {

	// Connect to local gRPC services, and keep connection alive until after
	// TearDown is called by using the fixture context.
	if rpcClient, err := rpc.Dial(s.FixtContext(), s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to connect to the local gRPC service on the DUT: ", err)
	} else {
		tf.fv.DUTRPCClient = rpcClient
	}

	tf.fv.ChromeService = chromeService.NewChromeServiceClient(tf.fv.DUTRPCClient.Conn)

	var signinProfileTestExtensionID string
	if tf.features.EnableHidScreenOnOobe {
		var ok bool
		signinProfileTestExtensionID, ok = s.Var(fixtureVarSigninKey)
		if !ok {
			s.Fatal("Failed to get sign-in key variable required for OOBE tests")
		}
	}

	// Start Chrome with the features and login mode provided by the test fixture.
	if _, err := tf.fv.ChromeService.New(ctx, &chromeService.NewRequest{
		LoginMode:       tf.features.LoginMode,
		EnableFeatures:  tf.features.EnableFeatures,
		DisableFeatures: tf.features.DisableFeatures,
		Credentials: &chromeService.NewRequest_Credentials{
			Username: defaultUsername,
			Password: defaultPassword,
		},
		EnableHidScreenOnOobe:        tf.features.EnableHidScreenOnOobe,
		SigninProfileTestExtensionId: signinProfileTestExtensionID,
	}); err != nil {
		s.Fatal("Failed to log into chrome on DUT: ", err)
	}

	return tf.fv
}

// Reset is called by the framework after each test (except for the last one) to
// do a light-weight reset of the environment to the original state.
//
// This is necessary to implement testing.FixtureImpl.
func (tf *fixture) Reset(ctx context.Context) (retErr error) {
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
	// Clean up chrome login state.
	if _, err := tf.fv.ChromeService.Close(ctx, &emptypb.Empty{}); err != nil {
		s.Error("Failed to close Chrome on the DUT: ", err)
	}

	// Close gRPC connection to DUT.
	if err := tf.fv.DUTRPCClient.Close(ctx); err != nil {
		s.Error("Failed to close gRPC connection to DUT: ", err)
	}
}
