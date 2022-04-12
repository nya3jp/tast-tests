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
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     fixture.LacrosPolicyLoggedIn,
		Desc:     "Fixture for a running FakeDMS with lacros",
		Contacts: []string{"mohamedaomar@google.com", "wtlee@chromium.org", "chromeos-commercial-remote-management@google.com"},
		Impl: lacrosfixt.NewComposedFixture(lacros.Rootfs, func(v lacrosfixt.FixtValue, pv interface{}) interface{} {
			return &struct {
				fakedms.HasFakeDMS
				lacrosfixt.FixtValue
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
				chrome.FakeLogin(chrome.Creds{User: "tast-user@managedchrome.com", Pass: "test0000"}),
				chrome.ExtraArgs("--disable-lacros-keep-alive")}
			return opts, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          fixture.PersistentLacros,
		Vars:            []string{lacrosfixt.LacrosDeployedBinary},
	})

	// TODO(b/218907052): Remove fixture after Journeys flag  is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name:     fixture.LacrosPolicyLoggedInFeatureJourneys,
		Desc:     "Fixture for a running FakeDMS with lacros and enabling the feature flag Journeys",
		Contacts: []string{"rodmartin@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: lacrosfixt.NewComposedFixture(lacros.Rootfs, func(v lacrosfixt.FixtValue, pv interface{}) interface{} {
			return &struct {
				fakedms.HasFakeDMS
				lacrosfixt.FixtValue
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
				chrome.FakeLogin(chrome.Creds{User: "tast-user@managedchrome.com", Pass: "test0000"}),
				chrome.ExtraArgs("--disable-lacros-keep-alive"),
				chrome.LacrosEnableFeatures("Journeys")}
			return opts, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          fixture.PersistentLacros,
		Vars:            []string{lacrosfixt.LacrosDeployedBinary},
	})

	testing.AddFixture(&testing.Fixture{
		Name:     fixture.LacrosPolicyLoggedInRealUser,
		Desc:     "Fixture for a running FakeDMS with lacros with a real managed user logged on",
		Contacts: []string{"anastasiian@chromium.org", "chromeos-commercial-remote-management@google.com"},
		Impl: lacrosfixt.NewComposedFixture(lacros.Rootfs, func(v lacrosfixt.FixtValue, pv interface{}) interface{} {
			return &struct {
				fakedms.HasFakeDMS
				lacrosfixt.FixtValue
			}{
				pv.(fakedms.HasFakeDMS),
				v,
			}
		}, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
			if !ok {
				return nil, errors.New("parent is not a FakeDMS fixture")
			}
			gaiaCreds, err := chrome.PickRandomCreds(s.RequiredVar("policy.ManagedUser.accountPool"))
			if err != nil {
				s.Fatal("Failed to parse managed user creds: ", err)
			}
			fdms.SetPersistentPolicyUser(&gaiaCreds.User)
			if err := fdms.WritePolicyBlob(policy.NewBlob()); err != nil {
				s.Fatal("Failed to write policies to FakeDMS: ", err)
			}
			opts := []chrome.Option{chrome.DMSPolicy(fdms.URL),
				chrome.GAIALogin(gaiaCreds),
				chrome.ExtraArgs("--disable-lacros-keep-alive")}
			return opts, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          fixture.PersistentLacros,
		Vars: []string{
			"policy.ManagedUser.accountPool",
			lacrosfixt.LacrosDeployedBinary,
		},
	})
}
