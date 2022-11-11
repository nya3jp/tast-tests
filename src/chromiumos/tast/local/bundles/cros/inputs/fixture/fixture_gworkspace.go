// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixture

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
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

const (
	workspaceResetTimeout    = 30 * time.Second
	workspacePreTestTimeout  = 20 * time.Second
	workspacepostTestTimeout = 20 * time.Second
)

// workspaceFixtureImpl implements testing.FixtureImpl.
type workspaceFixtureImpl struct {
	cr          *chrome.Chrome  // Underlying Chrome instance
	dm          deviceMode      // Device ui mode to test
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
		Desc: "Open google docs for inputs testing",
		Contacts: []string{
			"xiuwen@google.com",
		},
		Impl:            appFixture(tabletMode, true, browser.TypeAsh, GoogleDocs),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  workspaceResetTimeout,
		PostTestTimeout: workspacepostTestTimeout,
		ResetTimeout:    workspaceResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"ui.gaiaPoolDefault", "keepState"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: GoogleSlides,
		Desc: "Open google slides for inputs testing",
		Contacts: []string{
			"xiuwen@google.com",
		},
		Impl:            appFixture(tabletMode, true, browser.TypeAsh, GoogleSlides),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  workspaceResetTimeout,
		PostTestTimeout: workspacepostTestTimeout,
		ResetTimeout:    workspaceResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"ui.gaiaPoolDefault", "keepState"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: GoogleSheets,
		Desc: "Open google sheet for inputs testing",
		Contacts: []string{
			"xiuwen@google.com",
		},
		Impl:            appFixture(tabletMode, true, browser.TypeAsh, GoogleSheets),
		SetUpTimeout:    chrome.LoginTimeout,
		PreTestTimeout:  workspaceResetTimeout,
		PostTestTimeout: workspacepostTestTimeout,
		ResetTimeout:    workspaceResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"ui.gaiaPoolDefault", "keepState"},
	})

}

func appFixture(dm deviceMode, restart bool, browserType browser.Type, appName string, opts ...chrome.Option) testing.FixtureImpl {
	return &workspaceFixtureImpl{
		dm:          dm,
		restart:     restart,
		browserType: browserType,
		appName:     appName,
		fOpts:       opts,
	}
}

// WorkspaceFixtData is the data returned by SetUp and passed to tests.
type WorkspaceFixtData struct {
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.TestConn
	UserContext *useractions.UserContext
}

func (f *workspaceFixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var opts []chrome.Option

	switch f.dm {
	case tabletMode:
		opts = append(opts, chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	case clamshellMode:
		opts = append(opts, chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	}

	opts = append(opts, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
	opts = append(opts, chrome.VKEnabled())

	cr, err := browserfixt.NewChrome(ctx, f.browserType, lacrosfixt.NewConfig(), opts...)
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

	chrome.Lock()
	return WorkspaceFixtData{f.cr, f.tconn, f.uc}
}

func (f *workspaceFixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {
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

	workspaceAppReady(ctx, s, f.uc, f)
}

func (f *workspaceFixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *workspaceFixtureImpl) Reset(ctx context.Context) error {
	return nil
}

func (f *workspaceFixtureImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
}

func workspaceAppReady(ctx context.Context, s *testing.FixtTestState, uc *useractions.UserContext, f *workspaceFixtureImpl) {

	var conn *chrome.Conn
	var err error

	switch f.appName {
	case GoogleDocs:
		conn, err = f.cr.NewConn(ctx, cuj.NewGoogleDocsURL)
	case GoogleSheets:
		conn, err = f.cr.NewConn(ctx, cuj.NewGoogleSheetsURL)
	case GoogleSlides:
		conn, err = f.cr.NewConn(ctx, cuj.NewGoogleSlidesURL)

		ui := uiauto.New(f.tconn)
		ui.DoubleClick(nodewith.Name("title").First())(ctx)
	}

	if err != nil {
		s.Error(fmt.Sprintf("Failed to open %s: ", f.appName), err)
	}
	if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
		s.Error("Failed to wait for page to finish loading: ", err)
	}
	if err := cuj.MaximizeBrowserWindow(ctx, f.tconn, true, f.appName); err != nil {
		s.Error(fmt.Sprintf("Failed to maximize the %s page: ", f.appName), err)
	}
}
