// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	uifaillog "chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/logsaver"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            fixture.ChromePolicyLoggedIn,
		Desc:            "Logged into a user session",
		Contacts:        []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl:            &policyChromeFixture{},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.FakeDMS,
	})

	// ChromePolicyLoggedInLockscreen is identical to ChromePolicyLoggedIn, but will isolate test failures better.
	// TODO(b/231276590): Remove once ChromePolicyLoggedIn can clear the lockscreen.
	testing.AddFixture(&testing.Fixture{
		Name:            fixture.ChromePolicyLoggedInLockscreen,
		Desc:            "Logged into a user session and allow lockscreen to be used",
		Contacts:        []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl:            &policyChromeFixture{},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.FakeDMS,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     fixture.ChromePolicyLoggedInIsolatedApp,
		Desc:     "Logged into a user session with web app isolation enabled",
		Contacts: []string{"simonha@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &policyChromeFixture{
			extraOptsFunc: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				return []chrome.Option{chrome.EnableFeatures("WebAppEnableIsolatedStorage")}, nil
			},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.FakeDMS,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     fixture.ChromePolicyLoggedInFeatureChromeLabs,
		Desc:     "Logged into a user session with chrome labs enabled",
		Contacts: []string{"samicolon@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &policyChromeFixture{
			extraOptsFunc: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				return []chrome.Option{chrome.EnableFeatures("ChromeLabs")}, nil
			},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.FakeDMS,
	})

	// TODO(b/218907052): Remove fixture after Journeys flag  is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name:     fixture.ChromePolicyLoggedInFeatureJourneys,
		Desc:     "Logged into a user session with journeys enabled",
		Contacts: []string{"rodmartin@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &policyChromeFixture{
			extraOptsFunc: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				return []chrome.Option{chrome.EnableFeatures("Journeys")}, nil
			},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.FakeDMS,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     fixture.ChromeEnrolledLoggedIn,
		Desc:     "Logged into a user session with enrollment",
		Contacts: []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &policyChromeFixture{
			extraOptsFunc: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				return []chrome.Option{chrome.KeepEnrollment()}, nil
			},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.FakeDMSEnrolled,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     fixture.ChromeEnrolledLoggedInARC,
		Desc:     "Logged into a user session with enrollment with ARC support",
		Contacts: []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &policyChromeFixture{
			extraOptsFunc: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				return []chrome.Option{chrome.KeepEnrollment(), chrome.ARCEnabled(),
					chrome.ExtraArgs("--arc-availability=officially-supported")}, nil
			},
			waitForARC: true,
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.FakeDMSEnrolled,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     fixture.ChromeAdminDeskTemplatesLoggedIn,
		Desc:     "Logged into a user session with admin desk templates",
		Contacts: []string{"zhumatthew@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &policyChromeFixture{
			extraOptsFunc: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				return []chrome.Option{chrome.EnableFeatures("DesksTemplates")}, nil
			},
		},
		SetUpTimeout:    chrome.ManagedUserLoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.FakeDMS,
	})
}

type policyChromeFixture struct {
	// cr is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	cr *chrome.Chrome
	// fdms is the already running DMS server from the parent fixture.
	fdms *fakedms.FakeDMS

	// extraOptsFunc contains a callback to return extra options to pass to ash-chrome.
	extraOptsFunc chrome.OptionsCallback

	// waitForARC indicates the fixture needs to wait for ARC before login.
	// Only needs to be set if ARC is enabled.
	waitForARC bool

	// Marker for per-test log.
	logMarker *logsaver.Marker

	// clean stores if Chrome is clean after PostTest.
	// It is considered clean if it does not interfere with the next test, e.g. with a locked screen.
	clean bool
}

// FixtData is returned by the fixtures and used in tests
// by using interfaces HasChrome to get chrome and HashFakeDMS to get fakeDMS.
type FixtData struct {
	// fakeDMS is an already running DMS server.
	fakeDMS *fakedms.FakeDMS
	// chrome is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	chrome *chrome.Chrome
}

// NewFixtData returns a FixtData pointer with the given chrome and fdms instances.
// Needed as wilco fixtures use it to create a return value.
func NewFixtData(cr *chrome.Chrome, fdms *fakedms.FakeDMS) *FixtData {
	return &FixtData{fakeDMS: fdms, chrome: cr}
}

// Chrome implements the HasChrome interface.
func (f FixtData) Chrome() *chrome.Chrome {
	if f.chrome == nil {
		panic("Chrome is called with nil chrome instance")
	}
	return f.chrome
}

// FakeDMS implements the HasFakeDMS interface.
func (f FixtData) FakeDMS() *fakedms.FakeDMS {
	if f.fakeDMS == nil {
		panic("FakeDMS is called with nil fakeDMS instance")
	}
	return f.fakeDMS
}

// Credentials used for authenticating the test user.
const (
	Username = "tast-user@managedchrome.com"
	Password = "test0000"
)

// PolicyFileDump is the filename where the state of policies is dumped after the test ends.
const PolicyFileDump = "policies.json"

func (p *policyChromeFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	p.fdms = fdms

	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("Failed to open syslog reader: ", err)
	}
	defer reader.Close()

	opts := []chrome.Option{
		chrome.FakeLogin(chrome.Creds{User: Username, Pass: Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout),
		chrome.DeferLogin(),
	}

	if p.extraOptsFunc != nil {
		extraOpts, err := p.extraOptsFunc(ctx, s)
		if err != nil {
			s.Fatal("Failed to get extra options: ", err)
		}
		opts = append(opts, extraOpts...)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}

	logMarker, err := logsaver.NewMarker(cr.LogFilename())
	if err != nil {
		s.Error("Failed to start the log saver: ", err)
	}

	chromeOK := false
	defer func() {
		if !chromeOK {
			if err := cr.Close(ctx); err != nil {
				s.Error("Failed to close Chrome: ", err)
			}
		}
	}()

	defer uifaillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	if p.waitForARC {
		if arcType, ok := arc.Type(); ok && arcType == arc.Container {
			// The ARC mini instance, created when the login screen is
			// shown, blocks session_manager, preventing it from responding
			// to D-Bus methods. Cloud policy initialisation relies on being
			// able to contact session_manager, otherwise initialisation
			// will time out.
			err = arc.WaitAndroidInit(ctx, reader)
			if err != nil {
				s.Fatal("Failed waiting for Android init: ", err)
			}
		}
	}

	err = cr.ContinueLogin(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	p.cr = cr
	chromeOK = true

	chrome.Lock()

	if logMarker != nil {
		if err := logMarker.Save(filepath.Join(s.OutDir(), "chrome.fixture.log")); err != nil {
			s.Error("Failed to store per-test log data: ", err)
		}
	}

	return &FixtData{p.fdms, p.cr}
}

func (p *policyChromeFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if p.cr == nil {
		s.Fatal("Chrome not yet started")
	}

	if err := p.cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

	p.cr = nil
}

func (p *policyChromeFixture) Reset(ctx context.Context) error {
	// If Chrome not in a clean state, a failure here would invoke a TearDown and SetUp of
	// the fixture, ensuring a clean state for the next test.
	if !p.clean {
		return errors.New("failed to clean up Chrome after the last test")
	}

	// Check the connection to Chrome.
	if err := p.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}

	return nil
}

func (p *policyChromeFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if p.logMarker != nil {
		s.Error("A log marker is already created but not cleaned up")
	}
	logMarker, err := logsaver.NewMarker(p.cr.LogFilename())
	if err != nil {
		s.Error("Failed to start the log saver: ", err)
	} else {
		p.logMarker = logMarker
	}
}
func (p *policyChromeFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	p.clean = false

	if p.logMarker != nil {
		if err := p.logMarker.Save(filepath.Join(s.OutDir(), "chrome.log")); err != nil {
			s.Error("Failed to store per-test log data: ", err)
		}
		p.logMarker = nil
	}

	tconn, err := p.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create TestAPI connection: ", err)
	}

	policies, err := policyutil.PoliciesFromDUT(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain policies from Chrome: ", err)
	}

	b, err := json.MarshalIndent(policies, "", "  ")
	if err != nil {
		s.Fatal("Failed to marshal policies: ", err)
	}

	// The following checks are here in PostTest to associate any failures with the previous test.
	// A failure here generally means that something went wrong in the previous test,
	// or the test has insufficient cleanup.

	// Dump all policies as seen by Chrome to the tests OutDir.
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), PolicyFileDump), b, 0644); err != nil {
		s.Error("Failed to dump policies to file: ", err)
	}

	// The policy blob has already been cleared.
	if err := policyutil.RefreshChromePolicies(ctx, p.cr); err != nil {
		s.Fatal("Failed to clear policies: ", err)
	}

	// Reset Chrome state.
	if err := p.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed resetting existing Chrome session: ", err)
	}

	// Check that Chrome is not left with a locked screen.
	if st, err := lockscreen.GetState(ctx, tconn); err != nil {
		s.Fatal("Failed getting the lockscreen state: ", err)
	} else if st.Locked {
		s.Fatal("Unexpected lockscreen state after the test, the screen is locked")
	}

	// Check that no windows remain.
	if windows, err := ash.GetAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to get the windows: ", err)
	} else if len(windows) != 0 {
		s.Fatalf("Unexpected number of windows after the test; got %d, want 0", len(windows))
	}

	// Chrome should be in a good state to execute the next test.
	p.clean = true
}
