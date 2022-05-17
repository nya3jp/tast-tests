// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dgapi2

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "playBillingDgapi2Fixture",
		Desc: "The fixture builds on arcBootedForPlayBilling but ensures the Play Billing Dgapi PWA is started",
		Impl: &playBillingDgapi2Fixture{},
		Contacts: []string{
			"chromeos-apps-foundation-team@google.com",
			"jshikaram@chromium.org",
			"ashpakov@google.com", // until Sept 2022
		},
		Parent:          "arcBootedForPlayBilling",
		SetUpTimeout:    30 * time.Second,
		PreTestTimeout:  tryLimit * 2 * time.Minute, // 2 minutes per app installation attempt
		PostTestTimeout: 30 * time.Second,
	})
}

type playBillingDgapi2Fixture struct {
	TestApp *TestAppDgapi2
}

// The FixtDgapiData object is made available to users of this fixture via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.FixtValue().(dgapi2.FixtDgapiData)
//		...
//	}
type FixtDgapiData struct {
	Chrome  *chrome.Chrome
	TestApp *TestAppDgapi2
}

func (f *playBillingDgapi2Fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	p := s.ParentValue().(*arc.PreData)

	tconn, err := p.Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed getting Test API connection: ", err)
	}

	testApp, err := NewTestAppDgapi2(ctx, p.Chrome, p.UIDevice, tconn, p.ARC)
	if err != nil {
		s.Fatal("Failed trying to setup Dgapi2 test app: ", err)
	}

	f.TestApp = testApp

	return &FixtDgapiData{p.Chrome, testApp}
}

func (f *playBillingDgapi2Fixture) Reset(ctx context.Context) error {
	return nil
}

func (f *playBillingDgapi2Fixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	s.Logf("Installing %s", pkgName)
	if err := f.TestApp.InstallApp(ctx); err != nil {
		s.Fatal("Failed to install Dgapi2 test app: ", err)
	}

	if err := f.TestApp.Launch(ctx); err != nil {
		s.Fatal("Failed to launch Dgapi2 test app: ", err)
	}

	s.Log("Signing in")
	if err := f.TestApp.SignIn(ctx); err != nil {
		s.Fatal("Failed to sign into test app: ", err)
	}
}

func (f *playBillingDgapi2Fixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	trySaveLogs(ctx, f.TestApp)

	if err := f.TestApp.SignOut(ctx); err != nil {
		s.Fatal("Failed to sign out of test app: ", err)
	}

	if err := f.TestApp.Close(ctx); err != nil {
		s.Fatal("Failed to close Dgapi2 test app: ", err)
	}
}

func trySaveLogs(ctx context.Context, testApp *TestAppDgapi2) {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		testing.ContextLog(ctx, "Failed to get name of an out directory")
		return
	}

	logs, err := testApp.GetLogs(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get logs: ", err)
		return
	}

	path := filepath.Join(dir, "testapp_logs.txt")
	err = ioutil.WriteFile(path, []byte(strings.Join(logs, "\n")), 0644)
	if err != nil {
		testing.ContextLog(ctx, "Error writing logs to the file: ", err)
		return
	}
}

func (f *playBillingDgapi2Fixture) TearDown(ctx context.Context, s *testing.FixtState) {}
