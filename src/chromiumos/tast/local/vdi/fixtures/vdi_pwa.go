// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
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
		Name: fixture.PWACitrixLaunched,
		Desc: "Starts Chrome with Citrix WPA application started and logged in",
		Contacts: []string{
			"kamilszare@google.com",
			"cros-engprod-muc@google.com",
		},
		Impl: &pwaFixtureState{
			vdiConnector:   &citrix.Connector{},
			vdiServerKey:   "vdi.citrix_url",
			vdiUsernameKey: "vdi.citrix_username",
			vdiPasswordKey: "vdi.citrix_password",
		},
		Vars: []string{
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
		Name: fixture.PWAVmwareLaunched,
		Desc: "Starts Chrome with VMware PWA application and logged in",
		Contacts: []string{
			"kamilszare@google.com",
			"cros-engprod-muc@google.com",
		},
		Impl: &pwaFixtureState{
			vdiConnector:   &vmware.Connector{},
			vdiServerKey:   "vdi.vmware_url", // TODO: Add a separate key/value for the PWA Url in the private repo.
			vdiUsernameKey: "vdi.vmware_username",
			vdiPasswordKey: "vdi.vmware_password",
		},
		Vars: []string{
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

type pwaFixtureState struct {
	// cr is a connection to an already-started Chrome instance that loads
	// policies from FakeDMS.
	cr *chrome.Chrome
	// conn is the reference to the opened PWA
	conn *chrome.Conn
	// vdiPwaAddress is the address of the PWA app. Used when resetting state.
	// It is stored as well together with vdiServerKey as in Reset() we don't
	// have access to the FixtState to read variable.
	vdiPwaAddress string
	// vdiConnector
	vdiConnector vdiApps.VDIInt
	// vdiServerKey is a key to retrieve VDI server url.
	vdiServerKey string
	// vdiUsernameKey is a key to retrieve user name to access VDI app.
	vdiUsernameKey string
	// vdiPasswordKey is a key to retrieve VDI user password.
	vdiPasswordKey string
}

// PWAFixtureData is the type returned by vdi related fixtures. It holds the
// vdi connector providing to common actions that could be performed from the
// test side.
type PWAFixtureData struct {
	vdiConnector vdiApps.VDIInt
	cr           *chrome.Chrome
	uidetector   *uidetection.Context
	inKioskMode  bool
}

// VDIConnector returns VDI connector.
func (fd *PWAFixtureData) VDIConnector() vdiApps.VDIInt {
	return fd.vdiConnector
}

// Chrome returns Chrome. This adds support for chrome.HasChrome interface.
func (fd *PWAFixtureData) Chrome() *chrome.Chrome {
	return fd.cr
}

// UIDetector returns uidetector.
func (fd *PWAFixtureData) UIDetector() *uidetection.Context {
	return fd.uidetector
}

// InKioskMode returns boolean indicating whether it is a Kiosk session.
func (fd *PWAFixtureData) InKioskMode() bool {
	return fd.inKioskMode
}

func (v *pwaFixtureState) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: Username, Pass: Password}),
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

	v.vdiPwaAddress = s.RequiredVar(v.vdiServerKey)
	conn, err := cr.NewConn(ctx, v.vdiPwaAddress)
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	v.conn = conn

	//TODO: maximize the window

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	detector := uidetection.New(tconn,
		s.RequiredVar("uidetection.key_type"),
		s.RequiredVar("uidetection.key"),
		s.RequiredVar("uidetection.server"))
	v.vdiConnector.Init(s, detector)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	defer kb.Close()

	if err := v.vdiConnector.WebLogin(
		ctx,
		kb,
		&vdiApps.VDILoginConfig{
			Username: s.RequiredVar(v.vdiUsernameKey),
			Password: s.RequiredVar(v.vdiPasswordKey),
		}); err != nil {
		s.Fatal("Was not able to login to the VDI PWA application: ", err)
	}
	if err := v.vdiConnector.WaitForMainScreenVisible(ctx); err != nil {
		s.Fatal("Main screen of the VDI application was not visible: ", err)
	}

	chrome.Lock()
	// Mark that everything is okay and defer Chrome.Close() should not happen.
	ok = true
	return &FixtureData{vdiConnector: v.vdiConnector, cr: cr, uidetector: detector, inKioskMode: false}
}

func (v *pwaFixtureState) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	v.conn.Close() // TODO: Error handling

	if v.cr == nil {
		s.Fatal("Chrome not yet started")
	}

	if err := v.cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

	v.cr = nil
}

func (v *pwaFixtureState) Reset(ctx context.Context) error {
	// Check the connection to Chrome.
	if err := v.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}

	v.conn.Close() // TODO: Error handling
	conn, err := v.cr.NewConn(ctx, v.vdiPwaAddress)
	if err != nil {
		return errors.Wrap(err, "failed to connect to chrome")
	}
	v.conn = conn

	// Check the main VDI screen is on.
	if err := v.vdiConnector.WaitForMainScreenVisible(ctx); err != nil {
		return errors.Wrap(err, "VDi main screen was not present")
	}

	return nil
}

func (v *pwaFixtureState) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (v *pwaFixtureState) PostTest(ctx context.Context, s *testing.FixtTestState) {}
