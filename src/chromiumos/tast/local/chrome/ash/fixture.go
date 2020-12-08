// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "chromeLoggedInWith100DummyApps",
		Desc:            "Logged into a user session with 100 dummy apps",
		Impl:            &dummyAppsFixture{numApps: 100},
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "chromeLoggedInWith100DummyAppsSkiaRenderer",
		Desc:            "Logged into a user session with 100 dummy apps and skia renderer",
		Impl:            &dummyAppsFixture{numApps: 100, skiaRenderer: true},
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

type dummyAppsFixture struct {
	chromeLoggedIn testing.FixtureImpl
	extDirBase     string
	numApps        int
	skiaRenderer   bool
}

func (f *dummyAppsFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a tempdir: ", err)
	}
	f.extDirBase = extDirBase

	dirs, err := PrepareFakeApps(extDirBase, f.numApps)
	if err != nil {
		s.Fatal("Failed to prepare fake apps: ", err)
	}
	opts := make([]chrome.Option, 0, f.numApps)
	for _, dir := range dirs {
		opts = append(opts, chrome.UnpackedExtension(dir))
	}
	if f.skiaRenderer {
		opts = append(opts, chrome.EnableFeatures("UseSkiaRenderer"))
	}

	f.chromeLoggedIn = chrome.NewLoggedInFixture(opts...)
	return f.chromeLoggedIn.SetUp(ctx, s)
}

func (f *dummyAppsFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	f.chromeLoggedIn.TearDown(ctx, s)
	if err := os.RemoveAll(f.extDirBase); err != nil {
		s.Error("Failed to remove ", f.extDirBase, ": ", err)
	}
}

func (f *dummyAppsFixture) Reset(ctx context.Context) error {
	return f.chromeLoggedIn.Reset(ctx)
}

func (f *dummyAppsFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *dummyAppsFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
