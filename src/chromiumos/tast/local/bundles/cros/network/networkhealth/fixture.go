// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package networkhealth

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "networkHealth",
		Desc: "A network health mojo API is ready and available to use",
		Contacts: []string{
			"khegde@chromium.org",            // fixture author
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SetUpTimeout:    chrome.LoginTimeout + (30 * time.Second),
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 10 * time.Second,
		Impl:            &networkHealthFixture{},
	})
}

// networkHealthFixture implements testing.FixtureImpl.
type networkHealthFixture struct{}

func (f *networkHealthFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	return nil
}

func (f *networkHealthFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *networkHealthFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *networkHealthFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *networkHealthFixture) TearDown(ctx context.Context, s *testing.FixtState) {}
