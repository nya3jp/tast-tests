// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// PreData is returned by the precondition and used by tests to
// interact with Chrome and FakeDMS.
type PreData struct { // NOLINT
	// FakeDMS is an already running DMS  server.
	FakeDMS *fakedms.FakeDMS
	// Chrome is a connection to an already-started Chrome instance that loads policies from FakeDMS.
	Chrome *chrome.Chrome
}

// preImpl implements both testing.Precondition and testing.preconditionImpl.
type preImpl struct {
	fdms             *fakedms.FakeDMS
	cr               *chrome.Chrome
	fdmsDirectory    string
	fakeDMSCtxCancel func()
}

// newPrecondition creates a new precondition that can be shared by tests.
func newPrecondition() *preImpl {
	return &preImpl{}
}

// User starts Chrome and FakeDMS and allows tests to update the policies.
var User = newPrecondition()

func (p *preImpl) String() string         { return "user_policy" }
func (p *preImpl) Timeout() time.Duration { return 60 * time.Second }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing the current state that can be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	if p.fdms != nil && p.cr != nil {
		if err := policyutil.ResetChrome(ctx, p.fdms, p.cr); err == nil {
			return &PreData{p.fdms, p.cr}
		}

		// Cleanup failed; restart FakeDMS and Chrome for this test.
		p.fdms.Stop(ctx)
		chrome.Unlock()
		if err := p.cr.Close(ctx); err != nil {
			s.Log("Failed to close Chrome: ", err)
		}
	}

	ctx, st := timing.Start(ctx, "user_policy_precondition_setup")
	defer st.End()

	// Start FakeDMS.
	tmpdir, err := ioutil.TempDir("", "fdms-")
	if err != nil {
		s.Fatal("Failed to create fdms temp dir: ", err)
	}

	// TODO(crbug.com/1046244): Remove this and use correct context.
	fakeDMSCtx, fakeDMSCtxCancel := context.WithCancel(context.Background()) // NOLINT

	testing.ContextLogf(ctx, "FakeDMS starting in %s", tmpdir)
	fdms, err := fakedms.New(fakeDMSCtx, tmpdir)
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}

	p.fdms = fdms
	p.fakeDMSCtxCancel = fakeDMSCtxCancel
	p.fdmsDirectory = tmpdir

	pb := fakedms.NewPolicyBlob()
	if err := p.fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id"),
		chrome.DMSPolicy(fdms.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	p.cr = cr

	chrome.Lock()

	return &PreData{p.fdms, p.cr}
}

func (p *preImpl) copyFakeDMSLog(ctx context.Context, testDir string) error {
	return fsutil.CopyFile(filepath.Join(p.fdmsDirectory, "fakedms.log"), filepath.Join(testDir, "fakedms.log"))
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "user_policy_precondition_close")
	defer st.End()

	if p.fdms != nil {
		p.fdms.Stop(ctx)

		p.fakeDMSCtxCancel()

		// TODO(crbug.com/1049532): Copy log and policy.json for each test.
		if err := p.copyFakeDMSLog(ctx, s.OutDir()); err != nil {
			s.Error("Failed to copy FakeDMS log: ", err)
		}

		if err := os.RemoveAll(p.fdmsDirectory); err != nil {
			s.Error("Failed to remove FakeDMS directory: ", err)
		}
	}

	if p.cr != nil {
		chrome.Unlock()

		if err := p.cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome: ", err)
		}
	}

	p.fdms = nil
	p.cr = nil
}
