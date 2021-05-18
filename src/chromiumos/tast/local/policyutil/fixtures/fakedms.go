// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
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
		PostTestTimeout: 5 * time.Second,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "fakeDMSEnrolled",
		Desc:     "Fixture for a running FakeDMS",
		Contacts: []string{"vsavu@google.com", "chromeos-commercial-stability@google.com"},
		Impl: &fakeDMSFixture{
			importState: filepath.Join(fakedms.EnrollmentFakeDMSDir, fakedms.StateFile),
		},
		SetUpTimeout:    15 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		PostTestTimeout: 5 * time.Second,
		Parent:          "enrolled",
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "fakeDMSFamilyLink",
		Desc:     "Fixture for a running FakeDMS of Family Link account",
		Contacts: []string{"vsavu@google.com", "chromeos-commercial-stability@google.com"},
		Impl:     &fakeDMSFixture{},
		Vars: []string{
			"unicorn.childUser",
		},
		SetUpTimeout:    15 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		PostTestTimeout: 5 * time.Second,
	})
}

type fakeDMSFixture struct {
	// FakeDMS is the currently running fake DM server.
	fakeDMS *fakedms.FakeDMS
	// fdmsDir is the directory where FakeDMS is currently running.
	fdmsDir string
	// importState is the path to an existing state file for FakeDMS.
	importState string
}

func (f *fakeDMSFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	ctx, st := timing.Start(ctx, "fakeDMS_setup")
	defer st.End()

	// Use a tmpdir to ensure multiple startups don't override logs.
	tmpdir, err := ioutil.TempDir(s.OutDir(), "fdms-")
	if err != nil {
		s.Fatal("Failed to create fdms temp dir: ", err)
	}
	f.fdmsDir = tmpdir

	if f.importState != "" {
		if err := fsutil.CopyFile(f.importState, filepath.Join(f.fdmsDir, fakedms.StateFile)); err != nil {
			s.Fatalf("Failed to import the existing state from %q: %v", f.importState, err)
		}
	}

	// Start FakeDMS.
	fdms, err := fakedms.New(s.FixtContext(), f.fdmsDir)
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}

	// Make sure FakeDMS is running.
	if err := fdms.Ping(ctx); err != nil {
		s.Fatal("Failed to ping FakeDMS: ", err)
	}

	// Write empty policy blob.
	pb := fakedms.NewPolicyBlob()
	childUser := s.RequiredVar("unicorn.childUser")
	if len(childUser) > 0 {
		pb = &fakedms.PolicyBlob{
			PolicyUser:       childUser,
			ManagedUsers:     []string{"*"},
			InvalidationSrc:  16,
			InvalidationName: "test_policy",
		}
	}

	if err := fdms.WritePolicyBlob(pb); err != nil {
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
	if ctx.Err() != nil {
		s.Fatal("Context already expired: ", ctx.Err())
	}

	// Copy FakeDMS log to the current tests OutDir.
	src := filepath.Join(f.fdmsDir, fakedms.LogFile)
	dst := filepath.Join(s.OutDir(), fakedms.LogFile)
	if err := fsutil.CopyFile(src, dst); err != nil {
		s.Error("Failed to copy FakeDMS logs: ", err)
	}

	// Copy FakeDMS policies to the current tests OutDir.
	// Add prefix to avoid conflic with the Chrome fixture.
	src = filepath.Join(f.fdmsDir, fakedms.PolicyFile)
	dst = filepath.Join(s.OutDir(), "fakedms_"+fakedms.PolicyFile)
	if err := fsutil.CopyFile(src, dst); err != nil {
		s.Error("Failed to copy FakeDMS policies: ", err)
	}
}
