// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type localLeasedAccountFixture struct {
}

type LeasedAccountFixtureData struct {
	Username string
	Password string
}

func (t *localLeasedAccountFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Read the account data.
	data, err := ioutil.ReadFile("/tmp/account.json")
	if err != nil {
		s.Fatal("Failed to read account file: ", err)
	}

	// Parse the account data.
	var d struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		s.Fatal("Failed to unmarshal data: ", err)
	}

	return LeasedAccountFixtureData{
		Username: d.Username,
		Password: d.Password,
	}
}
func (t *localLeasedAccountFixture) Reset(ctx context.Context) error                        { return nil }
func (t *localLeasedAccountFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (t *localLeasedAccountFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
func (t *localLeasedAccountFixture) TearDown(ctx context.Context, s *testing.FixtState)     {}

const (
	accountGetFixtureTimeout              = time.Minute * 3
	baseRobloxAccountFixtureName          = "baseRobloxAccountFixture"
	RobloxFixtureAuthenticatedForAllTests = "robloxAuthenticatedFixture"
)

func init() {
	// TODO (b/222311973): This extra layer is needed to extend the remote fixture, pull out the account it wrote, and provide it to the next local fixture. If local fixtures could read fixture data from remotes, this wouldn't be needed at all.
	testing.AddFixture(&testing.Fixture{
		Name:            baseRobloxAccountFixtureName,
		Desc:            "Base fixture for Roblox tests which stores the leased account information",
		Contacts:        []string{"davidwelling@google.com", "arc-engprod@google.com"},
		Parent:          fixture.RobloxLeasedAccountFixture,
		Impl:            &localLeasedAccountFixture{},
		SetUpTimeout:    accountGetFixtureTimeout,
		TearDownTimeout: accountGetFixtureTimeout,
		ResetTimeout:    accountGetFixtureTimeout,
	})

	// The fixture which contains the authenticated account, and is set up with ARC++ and the play store authenticated.
	testing.AddFixture(&testing.Fixture{
		Name:     RobloxFixtureAuthenticatedForAllTests,
		Desc:     "Fixture for Roblox tests which sets up ARC, Play, and has leased account information",
		Contacts: []string{"davidwelling@google.com", "arc-engprod@google.com"},
		Parent:   baseRobloxAccountFixtureName,
		Impl: arc.NewArcBootedWithPlayStoreFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
				chrome.GAIALoginPool(s.RequiredVar("arcappgameperf.username") + ":" + s.RequiredVar("arcappgameperf.password")),
			}, nil
		}),
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute + accountGetFixtureTimeout,
		ResetTimeout:    30 * time.Second,
		PostTestTimeout: arc.PostTestTimeout,
		TearDownTimeout: 30*time.Second + accountGetFixtureTimeout,

		Vars: []string{"arcappgameperf.username", "arcappgameperf.password"},
	})
}
