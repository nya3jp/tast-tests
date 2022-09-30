// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink provides Family Link user login functions.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// resetTimeout is the timeout duration of trying to reset the current fixture.
const resetTimeout = 30 * time.Second

// NewFamilyLinkFixture creates a new implementation of the Family Link fixture.
func NewFamilyLinkFixture(parentUser, parentPassword, childUser, childPassword string, isOwner bool, opts ...chrome.Option) testing.FixtureImpl {
	return &familyLinkFixture{
		opts:           opts,
		parentUser:     parentUser,
		parentPassword: parentPassword,
		childUser:      childUser,
		childPassword:  childPassword,
		isOwner:        isOwner,
		isLacros:       false,
	}
}

// NewFamilyLinkFixtureLacros creates a new implementation of the Family Link fixture for Lacros.
func NewFamilyLinkFixtureLacros(parentUser, parentPassword, childUser, childPassword string, isOwner bool, opts ...chrome.Option) testing.FixtureImpl {
	return &familyLinkFixture{
		opts:           opts,
		parentUser:     parentUser,
		parentPassword: parentPassword,
		childUser:      childUser,
		childPassword:  childPassword,
		isOwner:        isOwner,
		isLacros:       true,
	}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkUnicornLogin",
		Desc:     "Supervised Family Link user login with Unicorn account",
		Contacts: []string{"tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Impl:     NewFamilyLinkFixture("family.parentEmail", "family.parentPassword", "family.unicornEmail", "family.unicornPassword", true),
		Vars: []string{
			"family.parentEmail",
			"family.parentPassword",
			"family.unicornEmail",
			"family.unicornPassword",
		},
		SetUpTimeout:    chrome.GAIALoginChildTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkUnicornLoginWithLacros",
		Desc:     "Supervised Family Link user login with Unicorn account",
		Contacts: []string{"galenemco@chromium.org", "cros-families-eng+test@google.com"},
		Impl:     NewFamilyLinkFixtureLacros("family.parentEmail", "family.parentPassword", "family.unicornEmail", "family.unicornPassword", true),
		Vars: []string{
			"family.parentEmail",
			"family.parentPassword",
			"family.unicornEmail",
			"family.unicornPassword",
		},
		SetUpTimeout:    chrome.GAIALoginChildTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkUnicornLoginNonOwner",
		Desc:     "Supervised Family Link user login with Unicorn account as second user on device",
		Contacts: []string{"tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Impl:     NewFamilyLinkFixture("family.parentEmail", "family.parentPassword", "family.unicornEmail", "family.unicornPassword", false),
		Vars: []string{
			"ui.gaiaPoolDefault",
			"family.parentEmail",
			"family.parentPassword",
			"family.unicornEmail",
			"family.unicornPassword",
		},
		SetUpTimeout:    chrome.GAIALoginChildTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkGellerLogin",
		Desc:     "Supervised Family Link user login with Geller account",
		Contacts: []string{"tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Impl:     NewFamilyLinkFixture("family.parentEmail", "family.parentPassword", "family.gellerEmail", "family.gellerPassword", true),
		Vars: []string{
			"family.parentEmail",
			"family.parentPassword",
			"family.gellerEmail",
			"family.gellerPassword",
		},
		SetUpTimeout:    chrome.GAIALoginChildTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkUnicornArcLogin",
		Desc:     "Supervised Family Link user login with Unicorn account and ARC support",
		Contacts: []string{"tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Impl:     NewFamilyLinkFixture("arc.parentUser", "arc.parentPassword", "arc.childUser", "arc.childPassword", true, chrome.ARCSupported()),
		Vars: []string{
			"arc.parentUser",
			"arc.parentPassword",
			"arc.childUser",
			"arc.childPassword",
		},
		SetUpTimeout:    chrome.GAIALoginChildTimeout + arc.BootTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkParentArcLogin",
		Desc:     "Non-supervised Family Link user login with regular parent account and ARC support",
		Contacts: []string{"tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Impl:     NewFamilyLinkFixture("arc.parentUser", "arc.parentPassword", "", "", true, chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...)),
		Vars: []string{
			"arc.parentUser",
			"arc.parentPassword",
		},
		SetUpTimeout:    chrome.GAIALoginTimeout + arc.BootTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkUnicornPolicyLogin",
		Desc:     "Supervised Family Link user login with Unicorn account and policy setup",
		Contacts: []string{"tobyhuang@chromium.org", "xiqiruan@chromium.org", "cros-families-eng+test@google.com"},
		Impl:     NewFamilyLinkFixture("family.parentEmail", "family.parentPassword", "family.unicornEmail", "family.unicornPassword", true),
		Vars: []string{
			"family.parentEmail",
			"family.parentPassword",
			"family.unicornEmail",
			"family.unicornPassword",
		},
		SetUpTimeout:    chrome.GAIALoginChildTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
		Parent:          fixture.PersistentFamilyLink,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "familyLinkUnicornArcPolicyLogin",
		Desc:     "Supervised Family Link user login with Unicorn account and ARC support with fakeDMS setup",
		Contacts: []string{"tobyhuang@chromium.org", "xiqiruan@chromium.org", "cros-families-eng+test@google.com"},
		Impl:     NewFamilyLinkFixture("arc.parentUser", "arc.parentPassword", "arc.childUser", "arc.childPassword", true, chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...)),
		Vars: []string{
			"arc.parentUser",
			"arc.parentPassword",
			"arc.childUser",
			"arc.childPassword",
		},
		SetUpTimeout:    chrome.GAIALoginChildTimeout + arc.BootTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
		Parent:          fixture.PersistentFamilyLinkARC,
	})
}

type familyLinkFixture struct {
	cr             *chrome.Chrome
	opts           []chrome.Option
	fdms           *fakedms.FakeDMS
	policyUser     string
	parentUser     string
	parentPassword string
	childUser      string
	childPassword  string
	isOwner        bool
	isLacros       bool
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	// Chrome is the running chrome instance.
	chrome *chrome.Chrome

	// FakeDMS is the running DMS server if any.
	fakeDMS *fakedms.FakeDMS

	// TestConn is a connection to the test extension.
	testConn *chrome.TestConn

	// PolicyUser is the user account used in the policy blob.
	policyUser string
}

// Chrome implements the HasChrome interface.
func (f FixtData) Chrome() *chrome.Chrome {
	if f.chrome == nil {
		panic("Chrome is called with nil chrome instance")
	}
	return f.chrome
}

// HasTestConn is an interface for fixture values that contain a TestConn instance. It allows
// retrieval of the underlying TestConn object.
type HasTestConn interface {
	TestConn() *chrome.TestConn
}

// TestConn implements the HasTestConn interface.
func (f FixtData) TestConn() *chrome.TestConn {
	if f.testConn == nil {
		panic("TestConn is called with nil testConn instance")
	}
	return f.testConn
}

// FakeDMS implements the HasFakeDMS interface.
func (f FixtData) FakeDMS() *fakedms.FakeDMS {
	if f.fakeDMS == nil {
		panic("FakeDMS is called with nil fakeDMS instance")
	}
	return f.fakeDMS
}

// HasPolicyUser is an interface for fixture values that contain a policy user. It allows
// retrieval of the underlying policy user string.
type HasPolicyUser interface {
	PolicyUser() string
}

// PolicyUser implements the HasPolicyUser interface.
func (f FixtData) PolicyUser() string {
	return f.policyUser
}

func (f *familyLinkFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	parentUser := s.RequiredVar(f.parentUser)
	parentPass := s.RequiredVar(f.parentPassword)

	// Dev tools are necessary for the test instrumentation to work, but by default
	// disabled for supervised users. Always force enable them in supervised users tests.
	f.opts = append(f.opts, chrome.ExtraArgs("--force-devtools-available"))

	isChildLogin := len(f.childUser) > 0 && len(f.childPassword) > 0
	if isChildLogin {
		childUser := s.RequiredVar(f.childUser)
		childPass := s.RequiredVar(f.childPassword)
		f.opts = append(f.opts, chrome.GAIALogin(chrome.Creds{
			User:       childUser,
			Pass:       childPass,
			ParentUser: parentUser,
			ParentPass: parentPass,
		}))
	} else {
		f.opts = append(f.opts, chrome.GAIALogin(chrome.Creds{
			User: parentUser,
			Pass: parentPass,
		}))
	}

	// Checks whether the current fixture has a FakeDMS parent fixture.
	fdms, isPolicyTest := s.ParentValue().(*fakedms.FakeDMS)
	if isPolicyTest {
		if err := fdms.Ping(ctx); err != nil {
			s.Fatal("Failed to ping FakeDMS: ", err)
		}

		if isChildLogin {
			f.policyUser = s.RequiredVar(f.childUser)
			// Family Link users look like consumer users
			// with @gmail.com emails but require policy.
			// Since policy key verification doesn't work
			// for gmail users, disable it.
			f.opts = append(f.opts, chrome.DisablePolicyKeyVerification())
		} else {
			f.policyUser = parentUser
		}

		f.opts = append(f.opts, chrome.DMSPolicy(fdms.URL))
	}

	if f.isLacros {
		var err error
		f.opts, err = lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(f.opts...)).Opts()
		if err != nil {
			s.Fatal("Failed to get lacros options: ", err)
		}
		f.opts = append(f.opts, chrome.EnableFeatures("LacrosForSupervisedUsers"))
	}

	if !f.isOwner {
		func() {
			// Log in and log out to create a user pod on the login screen.
			cr, err := chrome.New(ctx, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}
			defer cr.Close(ctx)

			if err := upstart.RestartJob(ctx, "ui"); err != nil {
				s.Fatal("Failed to restart ui: ", err)
			}
		}()

		// chrome.KeepState() is needed to show the login screen with a user pod (instead of the OOBE login screen).
		f.opts = append(f.opts, chrome.KeepState())
	}

	cr, err := chrome.New(ctx, f.opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if isPolicyTest {
		if err := policyutil.RefreshChromePolicies(ctx, cr); err != nil {
			s.Fatal("Failed to serve policies: ", err)
		}
	}

	f.cr = cr
	f.fdms = fdms
	fixtData := &FixtData{
		chrome:     cr,
		fakeDMS:    fdms,
		testConn:   tconn,
		policyUser: f.policyUser,
	}

	// Lock chrome after all Setup is complete so we don't block other fixtures.
	chrome.Lock()

	return fixtData
}

func (f *familyLinkFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if f.fdms != nil {
		f.fdms.Stop(ctx)
		f.fdms = nil
	}
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *familyLinkFixture) Reset(ctx context.Context) error {
	if f.fdms != nil {
		pb := policy.NewBlob()
		pb.PolicyUser = f.policyUser
		if err := policyutil.ResetChromeWithBlob(ctx, f.fdms, f.cr, pb); err != nil {
			return errors.Wrap(err, "failed to reset chrome")
		}
		return nil
	}

	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}

	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}

	return nil
}

func (f *familyLinkFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *familyLinkFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
