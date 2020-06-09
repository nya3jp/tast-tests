// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"os"
	"path"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	mountPointTimeout = 15 * time.Second // timeout waiting for CrosDisks to mount Drivefs.
	fuseIoTimeout     = 40 * time.Second // timeout waiting for the FUSE to be operational.
	resetTimeout      = 30 * time.Second // timeout for trying to reset the current precondition.
)

// PreData holds information made available to tests that specify preconditions.
type PreData struct {
	// Chrome is a connection to an already-started Chrome instance.
	// It cannot be closed by tests.
	Chrome *chrome.Chrome

	// The path to the Drivefs mount for the currently active user.
	MountPath string
}

// NewPrecondition creates a new Drivefs precondition for tests that need different args.
func NewPrecondition(name string, gaia *GaiaVars, extraArgs ...string) testing.Precondition {
	pre := &preImpl{
		name:      name,
		timeout:   resetTimeout + chrome.LoginTimeout,
		gaia:      gaia,
		extraArgs: extraArgs,
	}
	return pre
}

// GaiaVars holds the secret variables for username and password for a GAIA login.
type GaiaVars struct {
	UserVar string // the secret variable for the GAIA username
	PassVar string // the secret variable for the GAIA password
}

// preImpl implements both testing.Precondition and testing.preconditionImpl.
type preImpl struct {
	name    string        // testing.PreconditionImpl.String
	timeout time.Duration // testing.PreconditionImpl.Timeout

	extraArgs []string  // passed to Chrome on initialization
	gaia      *GaiaVars // a struct containing GAIA secret variables

	cr        *chrome.Chrome // Chrome instance
	mountPath string         // Drivefs mount path that is created
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing objects that can be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	func() {
		ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
		defer cancel()
		extraArgs := p.extraArgs

		if p.gaia == nil {
			s.Fatal("Failed as no GAIA login credentials were supplied")
		}

		username := s.RequiredVar(p.gaia.UserVar)
		password := s.RequiredVar(p.gaia.PassVar)
		var err error
		p.cr, err = chrome.New(
			ctx,
			chrome.GAIALogin(),
			chrome.Auth(username, password, "gaia-id"),
			chrome.ExtraArgs(extraArgs...),
		)

		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	}()

	normUser, err := session.NormalizeEmail(p.cr.User(), true)
	if err != nil {
		s.Fatal("Failed to normalize user name: ", err)
	}
	s.Log("Logged in as: ", normUser)

	// Check that cache folder was created by cryptohome.
	homePath, err := cryptohome.UserPath(ctx, normUser)
	if err != nil {
		s.Fatal("Failed to get home path: ", err)
	}
	cachePath := path.Join(homePath, "GCache", "v2")
	if dir, err := os.Stat(cachePath); !dir.IsDir() {
		s.Fatalf("Failed as cache dir %s is missing: %v", cachePath, err)
	}

	// It takes some time for request to mount Drive to be handled by CrosDisks
	// that creates the mount point. Poll for a mount point until timeout.
	if err := waitForMatchingMount(ctx, mountPointTimeout, isDriveFs); err != nil {
		s.Fatal("Failed with timeout while waiting for mountpoint creation: ", err)
	}
	mounts, err := findMatchingMount(isDriveFs)
	if err != nil {
		s.Fatal("Failed obtaining matching mounts: ", err)
	}
	if len(mounts) != 1 {
		s.Fatalf("Failed one drivefs mount expected found %d. Mounts found: %v", len(mounts), mounts)
	}
	mountPath := mounts[0].MountPath
	s.Log("drivefs is mounted into ", mountPath)

	// We expect to find at least this folder in the mount point.
	drivefsRoot := path.Join(mountPath, "root")

	// As drivefs may not be fully initialized yet all access to the mount point
	// may fail inside FUSE driver until the daemon is ready.
	// Poll for stat to succeed in case the drivefs daemon is never ready due to
	// some bug.
	if err := waitForMountConnected(ctx, fuseIoTimeout, drivefsRoot); err != nil {
		s.Fatal("Failed while waiting for stat: ", err)
	}
	dir, err := os.Stat(drivefsRoot)
	if err != nil {
		s.Fatalf("Failed trying to state %s: %v", drivefsRoot, err)
	}
	if !dir.IsDir() {
		s.Fatalf("Failed with no root folder inside %s: %v", mountPath, err)
	}
	s.Log("drivefs fully started")

	// Prevent the chrome package's New and Close functions from
	// being called while this precondition is active.
	chrome.Lock()

	return PreData{Chrome: p.cr, MountPath: mountPath}
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	chrome.Unlock()
}

func findMatchingMount(matcher func(sysutil.MountInfo) bool) (matches []sysutil.MountInfo, err error) {
	info, err := sysutil.MountInfoForPID(sysutil.SelfPID)
	if err != nil {
		return
	}
	for i := range info {
		if matcher(info[i]) {
			matches = append(matches, info[i])
		}
	}
	return
}

func waitForMatchingMount(ctx context.Context, timeout time.Duration, matcher func(sysutil.MountInfo) bool) error {
	testing.ContextLogf(ctx, "Waiting %v for a matching mount to appear", timeout)
	return testing.Poll(ctx, func(ctx context.Context) error {
		matches, err := findMatchingMount(matcher)
		if err != nil {
			return errors.Wrap(err, "IO error trying to list mounts")
		}
		if len(matches) == 0 {
			return errors.Wrap(os.ErrNotExist, "The matching mount was not found")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second})
}

func isDriveFs(info sysutil.MountInfo) bool {
	return info.Fstype == "fuse.drivefs"
}

func waitForMountConnected(ctx context.Context, timeout time.Duration, path string) error {
	testing.ContextLogf(ctx, "Waiting %v for mount to become connected", timeout)
	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := os.Stat(path)
		return err
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second})
}
