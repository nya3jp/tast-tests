// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"strings"

	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// BTPeersVar is the name of the tast var that specifies a comma-separated
// list of btpeer host addresses.
const BTPeersVar = "btpeers"

// TastVars are variables that BT fixtures may have. Can be used in fixture
// vars or test vars.
var TastVars = []string{
	BTPeersVar,
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithBluetoothRevampEnabled",
		Desc: "Logs into a user session with the BluetoothRevamp feature flag enabled",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            ChromeLoggedInWithBluetoothRevamp(true),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithBluetoothRevampDisabled",
		Desc: "Logs into a user session with the BluetoothRevamp feature flag disabled",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            ChromeLoggedInWithBluetoothRevamp(false),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithBluetoothEnabled",
		Desc: "Logs into a user session and enables Bluetooth during set up and tear down",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl:            &chromeLoggedInWithBluetoothEnabled{},
		Parent:          "chromeLoggedInWithBluetoothRevampEnabled",
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// ChromeLoggedInWithBluetoothRevamp returns a fixture implementation that
// builds on the existing chromeLoggedIn fixture to also enable or disable the
// BluetoothRevamp feature flag.
func ChromeLoggedInWithBluetoothRevamp(enabled bool) testing.FixtureImpl {
	return chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
		if enabled {
			return []chrome.Option{chrome.EnableFeatures("BluetoothRevamp")}, nil
		}
		return []chrome.Option{chrome.DisableFeatures("BluetoothRevamp")}, nil
	})
}

type chromeLoggedInWithBluetoothEnabled struct {
}

func (*chromeLoggedInWithBluetoothEnabled) Reset(ctx context.Context) error {
	if err := Enable(ctx); err != nil {
		return errors.Wrap(err, "failed to enable Bluetooth")
	}
	return nil
}

func (*chromeLoggedInWithBluetoothEnabled) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (*chromeLoggedInWithBluetoothEnabled) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (*chromeLoggedInWithBluetoothEnabled) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := Enable(ctx); err != nil {
		s.Fatal("Failed to enable Bluetooth: ", err)
	}
	return s.ParentValue()
}

func (*chromeLoggedInWithBluetoothEnabled) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := Enable(ctx); err != nil {
		s.Fatal("Failed to enable Bluetooth: ", err)
	}
}

// ConnectToBTPeers connects to the specified amount of btpeers and returns
// a list of chameleon.Chameleon controllers to use to interact with the
// btpeers' chameleond service.
//
// The "btpeers" test fixture var is parsed in-order for comma-separated btpeer
// host addresses. These host addresses must be resolvable from the dut, and
// since lab duts cannot usually resolve lab hostnames each address is likely
// required to be an IP.
//
// Only btpeers up to the required amount will be connected to. Once the
// required amount is reached successfully, any remaining known btpeers will
// be ignored. An error will be returned if there are not enough btpeers to meet
// the required amount or if it fails to connect to a btpeer.
func ConnectToBTPeers(ctx context.Context, btpeersVar string, requiredAmount int) ([]*chameleon.Chameleon, error) {
	btpeers := make([]*chameleon.Chameleon, 0)
	btpeerAddresses := strings.Split(btpeersVar, ",")
	for i, addr := range btpeerAddresses {
		if len(btpeers) >= requiredAmount {
			break
		}
		testing.ContextLogf(ctx, "Connecting to Chameleond btpeer #%d at %q", i+1, addr)
		btpeer, err := chameleon.New(ctx, addr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to connect to Chameleond btpeer #%d at %q", i+1, addr)
		}
		btpeers = append(btpeers, btpeer)
	}
	if len(btpeers) != requiredAmount {
		return nil, errors.Errorf("failed to connect to required amount of btpeers: expected %d, got %d", requiredAmount, len(btpeers))
	}
	return btpeers, nil
}
