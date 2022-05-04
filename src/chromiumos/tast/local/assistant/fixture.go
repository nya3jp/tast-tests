// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	setUpTimeout    = time.Minute
	tearDownTimeout = time.Minute
)

func init() {
	// Assistant fixtures use assistant test gaia as we have to make sure that
	// necessary bits are enabled to run our tests, e.g. device apps.
	//
	// Assistant Android support (e.g. open local Android app) requires Play
	// Store opt-in and device apps bit.
	//
	// TODO(b/231447154): Add withoutPlayStore fixture and add/use assistant
	// fixture from other assistant tast tests.
	testing.AddFixture(&testing.Fixture{
		Name: "assistantBaseWithPlayStore",
		Desc: "Assistant test gaia chrome session with Play Store",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Vars: []string{"assistant.username", "assistant.password"},
		Impl: arc.NewArcBootedWithPlayStoreFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.GAIALogin(chrome.Creds{
					User: s.RequiredVar("assistant.username"),
					Pass: s.RequiredVar("assistant.password"),
				}),
				VerboseLogging(),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
			}, nil
		}),
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		PostTestTimeout: arc.PostTestTimeout,
		ResetTimeout:    arc.ResetTimeout,
		TearDownTimeout: arc.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantWithArc",
		Desc: "Assistant is enabled with Arc",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent:          "assistantBaseWithPlayStore",
		Impl:            NewAssistantFixture(),
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})
}

type enabledFixture struct {
	cr *chrome.Chrome
	a  *arc.ARC
}

// FixtData is fixture data of assistant fixture.
type FixtData struct {
	Chrome *chrome.Chrome
	ARC    *arc.ARC
}

// NewAssistantFixture returns new assistant fixture.
func NewAssistantFixture() testing.FixtureImpl {
	return &enabledFixture{}
}

func (f *enabledFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	preData := s.ParentValue().(*arc.PreData)
	f.cr = preData.Chrome
	f.a = preData.ARC

	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if err := EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}

	return &FixtData{
		Chrome: f.cr,
		ARC:    f.a,
	}
}

func (f *enabledFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if err := Cleanup(ctx, s.HasError, f.cr, tconn); err != nil {
		s.Fatal("Failed to disable Assistant: ", err)
	}
}

func (f *enabledFixture) Reset(ctx context.Context) error {
	return nil
}
func (f *enabledFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (f *enabledFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
