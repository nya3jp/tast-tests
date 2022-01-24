// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
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
			vdiUsernameKey:        "vdi.citrix_username",
			vdiPasswordKey:        "vdi.citrix_password",
		},
		Vars: []string{
			"vdi.ota_citrix_username",
			"vdi.ota_citrix_password",
			"vdi.citrix_url",
			"vdi.citrix_username",
			"vdi.citrix_password",
			"uidetection.key_type",
			"uidetection.key",
			"uidetection.server",
		},
		SetUpTimeout:    chrome.EnrollmentAndLoginTimeout + vdiApps.VDILoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
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
		PostTestTimeout: 15 * time.Second,
		Data:            vmware.VmwareData,
	})
}

type fixtureState struct {
	// cr is a connection to an already-started Chrome instance that loads
	// policies from FakeDMS.
	cr *chrome.Chrome
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
	username := s.RequiredVar(v.usernameKey)
	password := s.RequiredVar(v.passwordKey)

	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
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

	detector := uidetection.New(tconn,
		s.RequiredVar("uidetection.key_type"),
		s.RequiredVar("uidetection.key"),
		s.RequiredVar("uidetection.server"),
	).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	v.vdiConnector.Init(s, detector)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	defer kb.Close()

	if err := v.vdiConnector.Login(
		ctx,
		kb,
		&vdiApps.VDILoginConfig{
			Server:   s.RequiredVar(v.vdiServerKey),
			Username: s.RequiredVar(v.vdiUsernameKey),
			Password: s.RequiredVar(v.vdiPasswordKey),
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

	// Check the main VDI screen is on.
	if err := v.vdiConnector.WaitForMainScreenVisible(ctx); err != nil {
		return errors.Wrap(err, "VDi main screen was not present")
	}

	return nil
}

func (v *fixtureState) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (v *fixtureState) PostTest(ctx context.Context, s *testing.FixtTestState) {}
