// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "fakeDMSWithLacrosEnabled",
		Desc:     "Fixture for a running FakeDMS with Lacros enabled",
		Contacts: []string{"mohamedaomar@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &fakeDMSFixture{
			extraPolicies: []policy.Policy{&policy.LacrosAvailability{Val: "side_by_side"}},
		},
		SetUpTimeout:    15 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		PostTestTimeout: 5 * time.Second,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     fixture.LacrosPolicyLoggedIn,
		Desc:     "Fixture for a running FakeDMS with lacros",
		Contacts: []string{"mohamedaomar@google.com", "wtlee@chromium.org", "chromeos-commercial-remote-management@google.com"},
		Impl: launcher.NewComposedFixture(launcher.Rootfs, func(v launcher.FixtValue, pv interface{}) interface{} {
			return &struct {
				fakedms.HasFakeDMS
				launcher.FixtValue
			}{
				pv.(fakedms.HasFakeDMS),
				v,
			}
		}, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
			if !ok {
				return nil, errors.New("parent is not a FakeDMS fixture")
			}
			opts := []chrome.Option{chrome.DMSPolicy(fdms.URL),
				chrome.FakeLogin(chrome.Creds{User: "tast-user@managedchrome.com", Pass: "test0000"})}
			return opts, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          "fakeDMSWithLacrosEnabled",
		Vars:            []string{launcher.LacrosDeployedBinary},
	})
}
