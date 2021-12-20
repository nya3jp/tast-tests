// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixtures contains fixtures useful for Kiosk mode tests.
package fixtures

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: fixture.KioskAutoLaunchCleanup,
		Desc: "Fixture should be used when kioksmode.AutoLaunch() option is passed when creating Kiosk session using kioskmode.New()",
		Contacts: []string{
			"kamilszarek@google.com",
			"alt-modalities-stability@google.com",
		},
		Impl:            &kiosk{},
		TearDownTimeout: chrome.ManagedUserLoginTimeout,
		Parent:          fixture.FakeDMSEnrolled,
	})
}

type kiosk struct {
	fakeDMS *fakedms.FakeDMS
}

// FixtData is fixture return data.
type FixtData struct {
	fakeDMS *fakedms.FakeDMS
}

// Credentials used for authenticating the test user.
const (
	username = "tast-user@managedchrome.com"
	password = "test0000"
)

// FakeDMS implements the HasFakeDMS interface.
func (fd FixtData) FakeDMS() *fakedms.FakeDMS {
	if fd.fakeDMS == nil {
		panic("FakeDMS is called with nil fakeDMS instance")
	}
	return fd.fakeDMS
}

func (k *kiosk) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a fakeDMSEnrolled fixture")
	}
	k.fakeDMS = fdms
	return FixtData{fakeDMS: fdms}
}

func (k *kiosk) TearDown(ctx context.Context, s *testing.FixtState) {
	s.Log("Kiosk clean up: Starting Chrome to clean policies")
	cr, err := chrome.New(
		ctx,
		chrome.FakeLogin(chrome.Creds{User: username, Pass: password}),
		chrome.DMSPolicy(k.fakeDMS.URL),
		chrome.KeepEnrollment(),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	if err := policyutil.ServeAndRefresh(ctx, k.fakeDMS, cr, []policy.Policy{}); err != nil {
		s.Error("Could not serve and refresh policies: ", err)
	}
}

func (k *kiosk) Reset(ctx context.Context) error {
	return nil
}

func (k *kiosk) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (k *kiosk) PostTest(ctx context.Context, s *testing.FixtTestState) {}
