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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

const (
	setUpTimeout    = time.Minute
	tearDownTimeout = time.Minute
	preTestTimeout  = time.Minute
	postTestTimeout = time.Minute
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "assistantBase",
		Desc: "Chrome session for assistant testing",
		Contacts: []string{
			"yawano@google.com",
			"assitive-eng@google.com",
		},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				VerboseLogging(),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantBaseWithStartAudioDecoderOnDemand",
		Desc: "Chrome session for assistant testing with StartAssistantAudioDecoderOnDemand flag",
		Contacts: []string{
			"yawano@google.com",
			"assitive-eng@google.com",
		},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				VerboseLogging(),
				chrome.EnableFeatures("StartAssistantAudioDecoderOnDemand"),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantBaseWithLegacyLauncher",
		Desc: "Chrome session for assistant testing and productivity launcher disabled",
		Contacts: []string{
			"yawano@google.com",
			"assitive-eng@google.com",
		},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				VerboseLogging(),
				chrome.DisableFeatures("ProductivityLauncher"),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// Assistant fixtures use assistant test gaia for tests with Arc++ feature
	// as we have to make sure that necessary bits are enabled to run our tests,
	// e.g. device apps.
	//
	// Assistant Android support (e.g. open local Android app) requires Play
	// Store opt-in and device apps bit.
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
		Name: "assistant",
		Desc: "Assistant is enabled",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent: "assistantBase",
		Impl: NewAssistantFixture(func(s *testing.FixtState) FixtData {
			return FixtData{
				Chrome: s.ParentValue().(*chrome.Chrome),
			}
		}),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantWithStartAudioDecoderOnDemand",
		Desc: "Assistant is enabled",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent: "assistantBaseWithStartAudioDecoderOnDemand",
		Impl: NewAssistantFixture(func(s *testing.FixtState) FixtData {
			return FixtData{
				Chrome: s.ParentValue().(*chrome.Chrome),
			}
		}),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantClamshell",
		Desc: "Assistant is enabled in Clamshell mode",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent:          "assistant",
		Impl:            newTabletFixture(false),
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantWithLegacyLauncher",
		Desc: "Assistant is enabled with productivity launcher disabled",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent: "assistantBaseWithLegacyLauncher",
		Impl: NewAssistantFixture(func(s *testing.FixtState) FixtData {
			return FixtData{
				Chrome: s.ParentValue().(*chrome.Chrome),
			}
		}),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantClamshellWithLegacyLauncher",
		Desc: "Assistant is enabled in Clamshell mode with productivity launcher disabled",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent:          "assistantWithLegacyLauncher",
		Impl:            newTabletFixture(false),
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantWithArc",
		Desc: "Assistant is enabled with Arc",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent: "assistantBaseWithPlayStore",
		Impl: NewAssistantFixture(func(s *testing.FixtState) FixtData {
			preData := s.ParentValue().(*arc.PreData)
			return FixtData{
				Chrome: preData.Chrome,
				ARC:    preData.ARC,
			}
		}),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
	})
}

type tabletFixture struct {
	enabled bool
	cleanup func(ctx context.Context) error
}

func newTabletFixture(e bool) testing.FixtureImpl {
	return &tabletFixture{
		enabled: e,
	}
}

func (f *tabletFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fixtData := s.ParentValue().(*FixtData)
	cr := fixtData.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, f.enabled)
	if err != nil {
		s.Fatal("Failed to put into specified mode: ", err)
	}
	f.cleanup = cleanup

	// If a DUT switches from Tablet mode to Clamshell mode, it can take a while
	// until launcher gets settled down.
	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Failed to wait the launcher state Closed: ", err)
	}

	return fixtData
}

func (f *tabletFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.cleanup != nil {
		f.cleanup(ctx)
	}
}

func (f *tabletFixture) Reset(ctx context.Context) error {
	return nil
}
func (f *tabletFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (f *tabletFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

type parentFixtDataCallback func(s *testing.FixtState) FixtData

type enabledFixture struct {
	cr *chrome.Chrome
	cb parentFixtDataCallback
}

// FixtData is fixture data of assistant fixture.
type FixtData struct {
	Chrome *chrome.Chrome
	ARC    *arc.ARC
}

// NewAssistantFixture returns new assistant fixture.
func NewAssistantFixture(cb parentFixtDataCallback) testing.FixtureImpl {
	return &enabledFixture{
		cb: cb,
	}
}

func (f *enabledFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fixtData := f.cb(s)
	f.cr = fixtData.Chrome

	return &fixtData
}

func (f *enabledFixture) TearDown(ctx context.Context, s *testing.FixtState) {}
func (f *enabledFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *enabledFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if err := EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
}

func (f *enabledFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Run Cleanup in PostTest instead of TearDown as we want to capture a
	// screenshot if a test fails. Also a previous test might leave the launcher
	// open if it failed by missing an expected response. It can cause a
	// following test to fail. Disabling assistant will close the launcher.
	if err := Cleanup(ctx, s.HasError, f.cr, tconn); err != nil {
		s.Fatal("Failed to disable Assistant: ", err)
	}
}
