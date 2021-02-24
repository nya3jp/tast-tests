// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"encoding/base64"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const fixtureTimeout = 3 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "install100Apps",
		Desc:            "Install 100 fake apps in a temporary directory",
		Contacts:        []string{"mukai@chromium.org"},
		Impl:            &fakeAppsFixture{numApps: 100},
		SetUpTimeout:    fixtureTimeout,
		TearDownTimeout: fixtureTimeout,
		// Fixtures don't support external data yet, so icon files are supplied
		// through a runtime variable for now.
		// TODO(crbug.com/1127165): fix this.
		Vars: []string{"ash.fake_icon_data"},
	})
}

type fakeAppsFixture struct {
	extDirBase string
	numApps    int
}

func (f *fakeAppsFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a tempdir: ", err)
	}
	f.extDirBase = extDirBase

	var iconData []byte
	if iconStr, ok := s.Var("ash.fake_icon_data"); ok {
		var err error
		if iconData, err = base64.StdEncoding.DecodeString(iconStr); err != nil {
			s.Fatal("Failed to parse fake icon data")
		}
	}

	dirs, err := PrepareFakeApps(extDirBase, f.numApps, iconData)
	if err != nil {
		s.Fatal("Failed to prepare fake apps: ", err)
	}
	opts := make([]chrome.Option, 0, f.numApps)
	for _, dir := range dirs {
		opts = append(opts, chrome.UnpackedExtension(dir))
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
