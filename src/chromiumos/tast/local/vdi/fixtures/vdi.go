// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/tape"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	vdiApps "chromiumos/tast/local/vdi/apps"
	"chromiumos/tast/local/vdi/apps/citrix"
	"chromiumos/tast/local/vdi/apps/vmware"
	"chromiumos/tast/testing"
)

// CitrixUsernamePrefix is the prefix before username (i.e. cros-citrix\user_1).
const CitrixUsernamePrefix = "cros-citrix\\"

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: fixture.CitrixLaunched,
		Desc: "Starts DUT using TOA with Citrix application installed, started and logged in",
		Contacts: []string{
			"kamilszare@google.com",
			"cros-engprod-muc@google.com",
		},
		Impl: &fixtureState{
			vdiApplicationToStart: apps.Citrix,
			vdiConnector:          &citrix.Connector{},
			usernameKey:           "vdi.ota_citrix_username",
			passwordKey:           "vdi.ota_citrix_password",
			vdiServerKey:          "vdi.citrix_url",
			useTape:               true,
		},
		Vars: []string{
			tape.ServiceAccountVar,
			"vdi.ota_citrix_username",
			"vdi.ota_citrix_password",
			"vdi.citrix_url",
			"uidetection.key_type",
			"uidetection.key",
			"uidetection.server",
		},
		SetUpTimeout:    chrome.EnrollmentAndLoginTimeout + vdiApps.VDILoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 60 * time.Second,
		Data:            citrix.CitrixData,
	})

	testing.AddFixture(&testing.Fixture{
		Name: fixture.VmwareLaunched,
		Desc: "Starts DUT using TOA with VMware application installed, started and logged in",
		Contacts: []string{
			"kamilszare@google.com",
			"cros-engprod-muc@google.com",
		},
		Impl: &fixtureState{
			vdiApplicationToStart: apps.VMWare,
			vdiConnector:          &vmware.Connector{},
			usernameKey:           "vdi.ota_vmware_username",
			passwordKey:           "vdi.ota_vmware_password",
			vdiServerKey:          "vdi.vmware_url",
			vdiUsernameKey:        "vdi.vmware_username",
			vdiPasswordKey:        "vdi.vmware_password",
			useTape:               false,
		},
		Vars: []string{
			"vdi.ota_vmware_username",
			"vdi.ota_vmware_password",
			"vdi.vmware_url",
			"vdi.vmware_username",
			"vdi.vmware_password",
			"uidetection.key_type",
			"uidetection.key",
			"uidetection.server",
		},
		SetUpTimeout:    chrome.EnrollmentAndLoginTimeout + vdiApps.VDILoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 60 * time.Second,
		Data:            vmware.VmwareData,
	})
}

type fixtureState struct {
	// cr is a connection to an already-started Chrome instance that loads
	// policies from FakeDMS.
	cr *chrome.Chrome
	// keyboard is a reference to keyboard to be release in the TearDown.
	keyboard *input.KeyboardEventWriter
	// vdiApplicationToStart is the VDI application that is launched.
	vdiApplicationToStart apps.App
	// vdiConnector
	vdiConnector vdiApps.VDIInt
	// usernameKey is a key to retrieve ota username. That user is configured
	// to install and pin dedicated VDI extension.
	usernameKey string
	// passwordKey is a key to retrieve ota user password.
	passwordKey string
	// vdiServerKey is a key to retrieve VDI server url.
	vdiServerKey string
	// vdiUsernameKey is a key to retrieve user name to access VDI app.
	vdiUsernameKey string
	// vdiPasswordKey is a key to retrieve VDI user password.
	vdiPasswordKey string
	// useTape is a flag to use Tape for leasing a vdi account.
	useTape bool
	// tapeAccountManager is used for cleaning up Tape.
	tapeAccountManager *tape.GenericAccountManager
}

// FixtureData is the type returned by vdi related fixtures. It holds the vdi
// connector providing to common actions that could be performed from the test
// side.
type FixtureData struct {
	vdiConnector vdiApps.VDIInt
	cr           *chrome.Chrome
	uidetector   *uidetection.Context
	inKioskMode  bool
}

// VDIConnector returns VDI connector.
func (fd *FixtureData) VDIConnector() vdiApps.VDIInt {
	return fd.vdiConnector
}

// Chrome returns Chrome. This adds support for chrome.HasChrome interface.
func (fd *FixtureData) Chrome() *chrome.Chrome {
	return fd.cr
}

// UIDetector returns uidetector.
func (fd *FixtureData) UIDetector() *uidetection.Context {
	return fd.uidetector
}

// InKioskMode returns boolean indicating whether it is a Kiosk session.
func (fd *FixtureData) InKioskMode() bool {
	return fd.inKioskMode
}

// HasVDIConnector is an interface that can be attached to a type that returns
// vdi connector.
type HasVDIConnector interface {
	VDIConnector() vdiApps.VDIInt
}

// HasUIDetector is an interface that can be attached to a type that returns
// uidetector.
type HasUIDetector interface {
	UIDetector() *uidetection.Context
}

// IsInKioskMode is an interface that can be attached to a type that returns
// whether fixture runs in Kiosk mode.
type IsInKioskMode interface {
	InKioskMode() bool
}

func (v *fixtureState) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var vdiUsername, vdiPassword string

	otaUsername := s.RequiredVar(v.usernameKey)
	otaPassword := s.RequiredVar(v.passwordKey)
	vdiServer := s.RequiredVar(v.vdiServerKey)

	if v.useTape {
		// TODO(b/242841251): Refactor leasing tape accounts.
		// Create an account manager and lease a vdi test account for the specified timeout as there are several sets in the scope of this fixture.
		accHelper, acc, err := tape.NewGenericAccountManager(ctx, []byte(s.RequiredVar(tape.ServiceAccountVar)), tape.WithTimeout(30*60), tape.WithPoolID(tape.Citrix))
		if err != nil {
			s.Fatal("Failed to create an account manager and lease a Citrix account: ", err)
		}
		v.tapeAccountManager = accHelper
		vdiUsername = CitrixUsernamePrefix + acc.Username
		vdiPassword = acc.Password
	} else {
		vdiUsername = s.RequiredVar(v.vdiUsernameKey)
		vdiPassword = s.RequiredVar(v.vdiPasswordKey)
	}

	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{User: otaUsername, Pass: otaPassword}),
		chrome.ProdPolicy(),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	ok := false
	defer func(ctx context.Context) {
		if !ok {
			if err := cr.Close(ctx); err != nil {
				s.Error("Failed to close Chrome: ", err)
			}
		}
	}(ctx)

	v.cr = cr

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "vdi_fixt_ui_tree")

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Waiting for apps to be installed before launching")
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, v.vdiApplicationToStart.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for app to install: ", err)
	}

	s.Log("VDI: Starting VDI app")
	if err := apps.Launch(ctx, tconn, v.vdiApplicationToStart.ID); err != nil {
		s.Fatal("Failed to launch vdi app: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, v.vdiApplicationToStart.ID, time.Minute); err != nil {
		s.Fatal("The VDI app did not appear in shelf after launch: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	// Keep it fo closing.
	v.keyboard = kb

	detector := uidetection.New(tconn,
		s.RequiredVar("uidetection.key_type"),
		s.RequiredVar("uidetection.key"),
		s.RequiredVar("uidetection.server"),
	).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	v.vdiConnector.Init(s, tconn, detector, kb)

	if err := v.vdiConnector.Login(
		ctx,
		&vdiApps.VDILoginConfig{
			Server:   vdiServer,
			Username: vdiUsername,
			Password: vdiPassword,
		}); err != nil {
		s.Fatal("Was not able to login to the VDI application: ", err)
	}
	if err := v.vdiConnector.WaitForMainScreenVisible(ctx); err != nil {
		s.Fatal("Main screen of the VDI application was not visible: ", err)
	}

	chrome.Lock()
	// Mark that everything is okay and defer Chrome.Close() should not happen.
	ok = true
	return &FixtureData{vdiConnector: v.vdiConnector, cr: cr, uidetector: detector, inKioskMode: false}
}

func (v *fixtureState) TearDown(ctx context.Context, s *testing.FixtState) {
	// Use a shortened context to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 1*time.Minute)
	defer cancel()

	if v.useTape {
		v.tapeAccountManager.CleanUp(cleanupCtx)
	}

	if err := v.vdiConnector.Logout(ctx); err != nil {
		s.Error("Couldn't logout from the VDI application: ", err)
	}

	v.keyboard.Close()

	chrome.Unlock()

	if v.cr == nil {
		s.Fatal("Chrome not yet started")
	}

	if err := v.cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

	v.cr = nil
}

func (v *fixtureState) Reset(ctx context.Context) error {
	// Check the connection to Chrome.
	if err := v.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}

	return nil
}

func (v *fixtureState) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (v *fixtureState) PostTest(ctx context.Context, s *testing.FixtTestState) {
	tconn, err := v.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all open windows: ", err)
	}
	for _, w := range ws {
		if err := w.CloseWindow(ctx, tconn); err != nil {
			s.Logf("Warning: Failed to close window (%+v): %v", w, err)
		}
	}

	testing.ContextLog(ctx, "VDI: Restarting VDI app")
	if err := apps.Launch(ctx, tconn, v.vdiApplicationToStart.ID); err != nil {
		s.Fatal("Failed to launch vdi app: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, v.vdiApplicationToStart.ID, time.Minute); err != nil {
		s.Fatal("The VDI app did not appear in shelf after launch: ", err)
	}

	if err := v.vdiConnector.LoginAfterRestart(ctx); err != nil {
		s.Fatal("Couldn't log in after restart: ", err)
	}

	if err := v.vdiConnector.WaitForMainScreenVisible(ctx); err != nil {
		s.Fatal("VDI main screen was not present: ", err)
	}
}
