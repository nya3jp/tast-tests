// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "chromeLoggedInRootfsRemoved",
		Desc:            "Removes rootfs verification, if required reboots the DUT and return logged into user session",
		Contacts:        []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Impl:            &loggedIn{},
		Parent:          "rootfsRemovedFixture",
		SetUpTimeout:    5 * time.Minute,
		ResetTimeout:    10 * time.Second,
		TearDownTimeout: 10 * time.Second,
	})
}

type Value struct {
	cr *chrome.Chrome
}

// loggedIn is a fixture to start Chrome.
type loggedIn struct {
	v *Value
}

func (f *loggedIn) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	f.v = &Value{cr: cr}
	return cr
}

func (f *loggedIn) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.v.cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}
	f.v.cr = nil
}

func (f *loggedIn) Reset(ctx context.Context) error {
	if err := f.v.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.v.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *loggedIn) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// No-op.
}

func (f *loggedIn) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// No-op.
}
