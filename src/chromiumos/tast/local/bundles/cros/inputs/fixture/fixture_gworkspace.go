// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixture

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

// app's name
const (
	GoogleDocs   = "googleDocs"
	GoogleSheets = "googleSheets"
	GoogleSlides = "googleSlides"
)

// appFixtureImpl implements testing.FixtureImpl.
type appFixtureImpl struct {
	cr          *chrome.Chrome  // Underlying Chrome instance
	dm          deviceMode      // Device ui mode to test
	vkEnabled   bool            // Whether virtual keyboard is force enabled
	restart     bool            // Whether restart the fixture after each test
	browserType browser.Type    // Whether Ash or Lacros is used for test
	appName     string          // name of testing app
	fOpts       []chrome.Option // Options that are passed to chrome.New
	tconn       *chrome.TestConn
	recorder    *uiauto.ScreenRecorder
	uc          *useractions.UserContext
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: GoogleDocs,
		Desc: "Test Google Docs with different inputs method",
		Contacts: []string{
			"xiuwen@google.com",
		},
		Impl:            appFixture(tabletMode, true, true, browser.TypeAsh, GoogleDocs),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"ui.gaiaPoolDefault", "keepState"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: GoogleSlides,
		Desc: "Test Google Docs with different inputs method",
		Contacts: []string{
			"xiuwen@google.com",
		},
		Impl:            appFixture(tabletMode, true, true, browser.TypeAsh, GoogleSlides),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"ui.gaiaPoolDefault", "keepState"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: GoogleSheets,
		Desc: "Test Google Sheets with different inputs method",
		Contacts: []string{
			"xiuwen@google.com",
		},
		Impl:            appFixture(tabletMode, true, true, browser.TypeAsh, GoogleSheets),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"ui.gaiaPoolDefault", "keepState"},
	})

}

func appFixture(dm deviceMode, vkEnabled, restart bool, browserType browser.Type, appName string, opts ...chrome.Option) testing.FixtureImpl {
	return &appFixtureImpl{
		dm:          dm,
		vkEnabled:   vkEnabled,
		restart:     restart,
		browserType: browserType,
		appName:     appName,
		fOpts:       opts,
	}
}

// GworkspaceFixtData is the data returned by SetUp and passed to tests.
type GworkspaceFixtData struct {
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.TestConn
	UserContext *useractions.UserContext
}

func (f *appFixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {

	var opts []chrome.Option

	switch f.dm {
	case tabletMode:
		opts = append(opts, chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	case clamshellMode:
		opts = append(opts, chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	}

	opts = append(opts, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
	opts = append(opts, chrome.VKEnabled())

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	f.cr = cr

	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection")
	}

	f.tconn = tconn

	uc, err := inputactions.NewInputsUserContextWithoutState(ctx, "", s.OutDir(), f.cr, f.tconn, nil)
	if err != nil {
		s.Fatal("Failed to create new inputs user context: ", err)
	}
	f.uc = uc

	openGoogleWorkspace(ctx, s, uc, f, tconn, cr, opts)

	chrome.Lock()
	return GworkspaceFixtData{f.cr, f.tconn, f.uc}
}

func (f *appFixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {
	f.uc.SetTestName(s.TestName())

	recorder, err := uiauto.NewScreenRecorder(ctx, f.tconn)
	if err != nil {
		s.Log("Failed to create screen recorder: ", err)
		return
	}
	if err := recorder.Start(ctx, f.tconn); err != nil {
		s.Log("Failed to start screen recorder: ", err)
		return
	}
	f.recorder = recorder
}

func (f *appFixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	util.ClickEnterToStartNewLine(ctx)
}

func (f *appFixtureImpl) Reset(ctx context.Context) error {
	return nil
}

func (f *appFixtureImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
}

func openGoogleWorkspace(ctx context.Context, s *testing.FixtState, uc *useractions.UserContext, f *appFixtureImpl, tconn *chrome.TestConn, cr *chrome.Chrome, opts []chrome.Option) {

	var conn *chrome.Conn
	var err error

	switch f.appName {
	case GoogleDocs:
		conn, err = cr.NewConn(ctx, cuj.NewGoogleDocsURL)
	case GoogleSheets:
		conn, err = cr.NewConn(ctx, cuj.NewGoogleSheetsURL)

	case GoogleSlides:
		conn, err = cr.NewConn(ctx, cuj.NewGoogleSlidesURL)

		titleNode := nodewith.Name("title").First()
		ui := uiauto.New(tconn)

		uiauto.Combine("open a new presentation",
			ui.DoubleClick(titleNode),
		)(ctx)
	}

	if err != nil {
		s.Error(fmt.Sprintf("Failed to open %s: ", f.appName), err)
	}
	if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
		s.Error("Failed to wait for page to finish loading: ", err)
	}
	if err := cuj.MaximizeBrowserWindow(ctx, tconn, true, f.appName); err != nil {
		s.Error(fmt.Sprintf("Failed to maximize the %s page: ", f.appName), err)
	}
}
