// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const fixtureTimeout = 3 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "install100Apps",
		Desc:            "Install 100 fake apps in a temporary directory",
		Contacts:        []string{"mukai@chromium.org"},
		Impl:            &defaultFakeAppFixture{numApps: 100},
		SetUpTimeout:    fixtureTimeout,
		TearDownTimeout: fixtureTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "install2Apps",
		Desc:            "Install 2 fake apps in a temporary directory",
		Contacts:        []string{"mukai@chromium.org"},
		Impl:            &defaultFakeAppFixture{numApps: 2},
		SetUpTimeout:    fixtureTimeout,
		TearDownTimeout: fixtureTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "install5AppsWithNames",
		Desc:            "Install 5 fake apps with the specified names in a temporary directory",
		Contacts:        []string{"andrewxu@chromium.org"},
		Impl:            &fakeAppWithNameFixture{names: FakeAppAlphabeticalNames},
		SetUpTimeout:    fixtureTimeout,
		TearDownTimeout: fixtureTimeout,
	})
}

// The base fixture to prepare for fake apps.
type fakeAppsFixtureBase struct {
	extDirBase string
}

func (f *fakeAppsFixtureBase) PrepareExtDir(s *testing.FixtState) error {
	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		return errors.Wrap(err, "failed to create a tempdir")
	}
	f.extDirBase = extDirBase
	return nil
}

func (f *fakeAppsFixtureBase) UnpackExtension(dirs []string) interface{} {
	opts := make([]chrome.Option, 0, len(dirs))
	for _, dir := range dirs {
		opts = append(opts, chrome.UnpackedExtension(dir))
	}
	return opts
}

func (f *fakeAppsFixtureBase) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := os.RemoveAll(f.extDirBase); err != nil {
		s.Error("Failed to remove ", f.extDirBase, ": ", err)
	}
}

func (f *fakeAppsFixtureBase) Reset(ctx context.Context) error {
	return nil
}

func (f *fakeAppsFixtureBase) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fakeAppsFixtureBase) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// The fixture used for populating default fake apps.
type defaultFakeAppFixture struct {
	fakeAppsFixtureBase
	numApps int
}

func (f *defaultFakeAppFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := f.PrepareExtDir(s); err != nil {
		s.Fatal("Failed to prepare for an external directory: ", err)
	}

	dirs, err := PrepareDefaultFakeApps(f.extDirBase, f.numApps, true)
	if err != nil {
		s.Fatal("Failed to prepare fake apps with default names and icons: ", err)
	}
	return f.UnpackExtension(dirs)
}

// The fixture used for populating fake apps with specified names.
type fakeAppWithNameFixture struct {
	fakeAppsFixtureBase
	names []string
}

func (f *fakeAppWithNameFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := f.PrepareExtDir(s); err != nil {
		s.Fatal("Failed to prepare for an external directory: ", err)
	}

	dirs, err := PrepareFakeAppsWithNames(f.extDirBase, f.names)
	if err != nil {
		s.Fatal("Failed to prepare fake apps with default names and icons: ", err)
	}
	return f.UnpackExtension(dirs)
}
