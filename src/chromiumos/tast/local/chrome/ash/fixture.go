// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/testing"
)

const fixtureTimeout = 3 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "install100Apps",
		Desc:            "Install 100 fake apps in a temporary directory",
		Contacts:        []string{"mukai@chromium.org"},
		Impl:            &fakeAppsFixture{numApps: 100, bt: browser.TypeAsh},
		SetUpTimeout:    fixtureTimeout,
		TearDownTimeout: fixtureTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "install2Apps",
		Desc:            "Install 2 fake apps in a temporary directory",
		Contacts:        []string{"mukai@chromium.org"},
		Impl:            &fakeAppsFixture{numApps: 2, bt: browser.TypeAsh},
		SetUpTimeout:    fixtureTimeout,
		TearDownTimeout: fixtureTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "install100LacrosApps",
		Desc:            "Install 100 fake apps from lacros extensions in a temporary directory",
		Contacts:        []string{"hyungtaekim@chromium.org"},
		Impl:            &fakeAppsFixture{numApps: 100, bt: browser.TypeLacros},
		SetUpTimeout:    fixtureTimeout,
		TearDownTimeout: fixtureTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "install2LacrosApps",
		Desc:            "Install 2 fake apps from lacros extensions in a temporary directory",
		Contacts:        []string{"hyungtaekim@chromium.org"},
		Impl:            &fakeAppsFixture{numApps: 2, bt: browser.TypeLacros},
		SetUpTimeout:    fixtureTimeout,
		TearDownTimeout: fixtureTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "install100AppsWithFileHandlers",
		Desc: "TBD",
		Contacts: []string{"lucmult@chromium.org"},
		Impl: &fakeAppsFixture{numApps: 10, bt: browser.TypeAsh, manifestExtras: `
			//"file_browser_handlers": {
			//	"id": "upload",
			//	"default_title": "upload to test",
			//	"file_filters": [
			//		"*.txt"
			//	]
			//},
			"file_handlers": {
				"extensions": [ "txt" ]
			},
			"permissions": [{
				"fileBrowserHandler",
				"fileSystem": ["requestFileSystem", "write"]
			}],
			`},
		SetUpTimeout:    fixtureTimeout,
		TearDownTimeout: fixtureTimeout,
	})
}

type fakeAppsFixture struct {
	extDirBase string
	numApps    int
	bt         browser.Type
	manifestExtras string
}

func (f *fakeAppsFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a tempdir: ", err)
	}
	f.extDirBase = extDirBase

	dirs, err := PrepareDefaultFakeApps(extDirBase, generateFakeAppNames(f.numApps), true, f.manifestExtras)
	if err != nil {
		s.Fatal("Failed to prepare fake apps: ", err)
	}
	opts := make([]chrome.Option, 0, f.numApps)
	for _, dir := range dirs {
		if f.bt == browser.TypeLacros {
			opts = append(opts, chrome.LacrosUnpackedExtension(dir))
		} else {
			opts = append(opts, chrome.UnpackedExtension(dir))
		}
	}
	return opts
}

func (f *fakeAppsFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := os.RemoveAll(f.extDirBase); err != nil {
		s.Error("Failed to remove ", f.extDirBase, ": ", err)
	}
}

func (f *fakeAppsFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *fakeAppsFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fakeAppsFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
