// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"io/ioutil"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "fakeDMS",
		Desc:            "Fixture for a running FakeDMS",
		Contacts:        []string{"vsavu@google.com", "chromeos-commercial-stability@google.com"},
		Impl:            &fakeDMSFixture{},
		SetUpTimeout:    15 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
	})
}

type fakeDMSFixture struct {
	// FakeDMS is the currently running fake DM server.
	fakeDMS *fakedms.FakeDMS
}

func (f *fakeDMSFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	ctx, st := timing.Start(ctx, "fakeDMS_setup")
	defer st.End()

	// Use a tmpdir to ensure multiple startups don't overrite logs.
	tmpdir, err := ioutil.TempDir(s.OutDir(), "fdms-")
	if err != nil {
		s.Fatal("Failed to create fdms temp dir: ", err)
	}

	// Start FakeDMS.
	fdms, err := fakedms.New(s.FixtContext(), tmpdir)
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}

	// Make sure FakeDMS is running.
	if err := fdms.Ping(ctx); err != nil {
		s.Fatal("Failed to ping FakeDMS: ", err)
	}

	if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	f.fakeDMS = fdms

	return f.fakeDMS
}

func (f *fakeDMSFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	ctx, st := timing.Start(ctx, "fakeDMS_teardown")
	defer st.End()

	if f.fakeDMS != nil {
		f.fakeDMS.Stop(ctx)
	}
}

func (f *fakeDMSFixture) Reset(ctx context.Context) error {
	// Make sure FakeDMS is still running.
	if err := f.fakeDMS.Ping(ctx); err != nil {
		return errors.Wrap(err, "failed to ping FakeDMS")
	}

	// Write empty policy blob.
	if err := f.fakeDMS.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		return errors.Wrap(err, "failed to clear policies in FakeDMS")
	}

	return nil
}

func (f *fakeDMSFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (f *fakeDMSFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// TODO(crbug.com/1049532): Copy logs after each test finishes.
}
