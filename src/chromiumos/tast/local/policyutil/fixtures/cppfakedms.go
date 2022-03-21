// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            fixture.CPPFakeDMS,
		Desc:            "Fixture for a running FakeDMS",
		Contacts:        []string{"vsavu@google.com", "mohamedaomar@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl:            &cppfakeDMSFixture{},
		SetUpTimeout:    15 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		PostTestTimeout: 5 * time.Second,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     fixture.CPPFakeDMSEnrolled,
		Desc:     "Fixture for a running FakeDMS",
		Contacts: []string{"vsavu@google.com", "mohamedaomar@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl: &cppfakeDMSFixture{
			importState: filepath.Join(fakedms.EnrollmentFakeDMSDir, fakedms.StateFile),
		},
		SetUpTimeout:    15 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		PostTestTimeout: 5 * time.Second,
		Parent:          fixture.Enrolled,
	})
}

type cppfakeDMSFixture struct {
	// FakeDMS is the currently running fake DM server.
	fakeDMS *fakedms.FakeDMS
	// fdmsDir is the directory where FakeDMS is currently running.
	fdmsDir string
	// importState is the path to an existing state file for FakeDMS.
	importState string
}

func (f *cppfakeDMSFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
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
	fdms, err := fakedms.NewCpp(s.FixtContext(), f.fdmsDir)
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}

	// Make sure FakeDMS is running.
	if err := fdms.Ping(ctx); err != nil {
		s.Fatal("Failed to ping FakeDMS: ", err)
	}

	pb := policy.NewBlob()

	if err := fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	f.fakeDMS = fdms

	return f.fakeDMS
}

func (f *cppfakeDMSFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	ctx, st := timing.Start(ctx, "fakeDMS_teardown")
	defer st.End()

	if f.fakeDMS != nil {
		f.fakeDMS.Stop(ctx)
	}
}

func (f *cppfakeDMSFixture) Reset(ctx context.Context) error {
	// Make sure FakeDMS is still running.
	if err := f.fakeDMS.Ping(ctx); err != nil {
		return errors.Wrap(err, "failed to ping FakeDMS")
	}

	// Write policy blob.
	if err := f.fakeDMS.WritePolicyBlob(policy.NewBlob()); err != nil {
		return errors.Wrap(err, "failed to clear policies in FakeDMS")
	}

	return nil
}

func (f *cppfakeDMSFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}
func (f *cppfakeDMSFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
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

	// Copy external policies to the current tests OutDir.
	// Add prefix for consistency with copied policy file.
	src = filepath.Join(f.fdmsDir, fakedms.ExtensionPolicyDir)
	dst = filepath.Join(s.OutDir(), "fakedms_"+fakedms.ExtensionPolicyDir)
	if stat, err := os.Stat(src); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.Errorf("Unexpected error when trying to stat %s: %v", src, err)
		}
	} else if srcIsDir := stat.Mode()&os.ModeType == os.ModeDir; srcIsDir {
		if err := fsutil.CopyDir(src, dst); err != nil {
			s.Error("Failed to copy external FakeDMS policies: ", err)
		}
	}
}
