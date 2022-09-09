// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/logsaver"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedIn",
		Desc:     "Logged into a user session",
		Contacts: []string{"nya@chromium.org", "oka@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInDisableSync",
		Desc:     "Logged into a user session with --disable-sync flag",
		Contacts: []string{"dhaddock@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{ExtraArgs("--disable-sync")}, nil
		}),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInDisableSyncNoFwUpdate",
		Desc:     "Logged into a user session with --disable-sync flag and firmware updates disabled",
		Contacts: []string{"cwd@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{ExtraArgs("--disable-sync"), ExtraArgs("--disable-features=FirmwareUpdaterApp")}, nil
		}),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInForInputs",
		Desc:     "Logged into a user session for essential inputs",
		Contacts: []string{"shengjun@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInGuest",
		Desc:     "Logged into a guest user session",
		Contacts: []string{"benreich@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{GuestLogin()}, nil
		}),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInGuestForInputs",
		Desc:     "Logged into a guest user session for essential inputs",
		Contacts: []string{"shengjun@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{GuestLogin()}, nil
		}),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInWith100FakeApps",
		Desc:     "Logged into a user session with 100 fake apps",
		Contacts: []string{"mukai@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return nil, nil
		}),
		Parent:          "install100Apps",
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// TOOD(b/233238923): Remove when passthrough is enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInWith100FakeAppsPassthroughCmdDecoder",
		Desc:     "Logged into a user session with 100 fake apps and the passthrough command decoder enabled",
		Contacts: []string{"hob@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{EnableFeatures("DefaultPassthroughCommandDecoder")}, nil
		}),
		Parent:          "install100Apps",
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	// TODO(crbug.com/1351225): Move tests from this fixture to chromeLoggedInWith100FakeApps.
	// This fixture used to force-enable productivity launcher. Several tests seem to use it for
	// that reason and likely don't depend on app sorting behavior.
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInWith100FakeAppsNoAppSort",
		Desc:     "Logged into a user session with 100 fake apps and app sorting disabled",
		Contacts: []string{"jamescook@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{DisableFeatures("LauncherAppSort")}, nil
		}),
		Parent:          "install100Apps",
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInWith100FakeAppsAppSort",
		Desc:     "Logged into a user session with 100 fake apps and app sorting enabled",
		Contacts: []string{"andrewxu@chromium.org"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{EnableFeatures("LauncherAppSort")}, nil
		}),
		Parent:          "install100Apps",
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInWithCalendarView",
		Desc:     "Logged into a session with Gaia user where CalendarView is enabled",
		Contacts: []string{"jiamingc@google.com"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{GAIALoginPool(s.RequiredVar("calendar.googleCalendarAccountPool")), EnableFeatures("CalendarView")}, nil
		}),
		Vars:            []string{"calendar.googleCalendarAccountPool"},
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInWithGaia",
		Desc:     "Logged into a session with Gaia user",
		Contacts: []string{"jinrongwu@google.com"},
		Vars:     []string{"ui.gaiaPoolDefault"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault"))}, nil
		}),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInThunderbolt",
		Desc:     "Logged into a user session to support thunderbolt devices",
		Contacts: []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Vars:     []string{"ui.signinProfileTestExtensionManifestKey"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{DeferLogin(), LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey"))}, nil
		}),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInDisableFirmwareUpdaterApp",
		Desc:     "Logged into a user session with FirmwareUpdaterApp disabled",
		Contacts: []string{"ramsaroop@google.com", "chromeos-perfmetrics-eng@google.com"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{DisableFeatures("FirmwareUpdaterApp")}, nil
		}),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInWithOsFeedback",
		Desc:     "Logged into a user session with OS Feedback enabled",
		Contacts: []string{"michaelcheco@google.com"},
		Impl: NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]Option, error) {
			return []Option{EnableFeatures("OsFeedback")}, nil
		}),
		SetUpTimeout:    LoginTimeout,
		ResetTimeout:    ResetTimeout,
		TearDownTimeout: ResetTimeout,
	})
}

// OptionsCallback is the function used to set up the fixture by returning Chrome options.
type OptionsCallback func(ctx context.Context, s *testing.FixtState) ([]Option, error)

// loggedInFixture is a fixture to start Chrome with the given options.
// If the parent is specified, and the parent returns a value of []Option, it
// will also add those options when starting Chrome.
type loggedInFixture struct {
	cr        *Chrome
	fOpt      OptionsCallback  // Function to generate Chrome Options
	logMarker *logsaver.Marker // Marker for per-test log.
}

// NewLoggedInFixture returns a FixtureImpl with a OptionsCallback function to provide Chrome options.
func NewLoggedInFixture(fOpt OptionsCallback) testing.FixtureImpl {
	return &loggedInFixture{fOpt: fOpt}
}

func (f *loggedInFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var opts []Option
	// If there's a parent fixture and the fixture supplies extra options, use them.
	if extraOpts, ok := s.ParentValue().([]Option); ok {
		opts = append(opts, extraOpts...)
	}

	crOpts, err := f.fOpt(ctx, s)
	if err != nil {
		s.Fatal("Failed to obtain Chrome options: ", err)
	}
	opts = append(opts, crOpts...)

	cr, err := New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	Lock()
	f.cr = cr
	return cr
}

func (f *loggedInFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *loggedInFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *loggedInFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if f.logMarker != nil {
		s.Log("A log marker is already created but not cleaned up")
	}
	logMarker, err := logsaver.NewMarker(f.cr.LogFilename())
	if err == nil {
		f.logMarker = logMarker
	} else {
		s.Log("Failed to start the log saver: ", err)
	}
}

func (f *loggedInFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.logMarker != nil {
		if err := f.logMarker.Save(filepath.Join(s.OutDir(), "chrome.log")); err != nil {
			s.Log("Failed to store per-test log data: ", err)
		}
		f.logMarker = nil
	}
}
