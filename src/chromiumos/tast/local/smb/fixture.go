// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package smb

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	smbdSetupTimeout = 5 * time.Second
	setUpTimeout     = chrome.LoginTimeout + smbdSetupTimeout
	tearDownTimeout  = chrome.ResetTimeout
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "smbStarted",
		Desc:            "Samba server started with 2 shares ready",
		Contacts:        []string{"chromeos-files-syd@chromium.org", "benreich@chromium.org"},
		Parent:          "chromeLoggedIn",
		Impl:            &fixture{},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    smbdSetupTimeout,
		TearDownTimeout: tearDownTimeout,
	})
}

// FixtureData is the struct exposed to tests.
type FixtureData struct {
	Chrome         *chrome.Chrome
	Server         *Server
	GuestSharePath string
}

type fixture struct {
	cr       *chrome.Chrome
	bconn    *chrome.Conn
	server   *Server
	guestDir string
	tempDir  string
}

// SetUp runs at the start of all the tests OR if a test unsuccessfully runs.
// If SetUp is ran after an unsuccessful test, Reset should clear the state to
// allow it to be setup again.
func (f *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	success := false
	f.cr = s.ParentValue().(*chrome.Chrome)
	defer func() {
		if !success {
			f.cr = nil
		}
	}()

	// Connect to Files app background page.
	backgroundMatcher := func(t *chrome.Target) bool {
		return t.URL == "chrome-extension://"+apps.Files.ID+"/background.html"
	}
	bconn, err := f.cr.NewConnForTarget(ctx, backgroundMatcher)
	if err != nil {
		s.Fatal("Failed to get connection to Files app background page: ", err)
	}
	f.bconn = bconn
	defer func() {
		if !success {
			if err := f.bconn.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close background page connection: ", err)
			}
			f.bconn = nil
		}
	}()

	// Create a temporory directory for smb.conf and a guestshare, this should be
	// removed at the end of the fixture or in the event of an error.
	dir, err := ioutil.TempDir("", "temporary_guestshare")
	if err != nil {
		s.Fatal("Failed to create temporary directory for shares: ", err)
	}
	f.tempDir = dir
	defer func() {
		if !success {
			os.RemoveAll(dir)
		}
	}()

	// The enclosing directory must be accessible by samba otherwise the daemon
	// won't be able to chdir into the guestshare.
	if err := os.Chmod(dir, 0777); err != nil {
		s.Fatal("Failed to update chmod to 0777 for temp folder: ", err)
	}

	guestDir := filepath.Join(dir, "guestshare")
	if err := os.MkdirAll(guestDir, 0777); err != nil {
		s.Fatal("Failed to create guestshare: ", err)
	}

	guestSambaConf, err := CreateGuestSambaConf(ctx, guestDir, dir)
	if err != nil {
		s.Fatal("Failed to create guest samba configuration: ", err)
	}

	// Start the smbd process which will dump an error log if it is not
	// terminated by a SIGTERM.
	server := NewServer(ctx, guestSambaConf)
	if err := server.Start(ctx); err != nil {
		s.Fatal("Failed to start smbd daemon: ", err)
	}
	f.server = server

	success = true
	return FixtureData{
		Chrome:         f.cr,
		Server:         server,
		GuestSharePath: guestDir,
	}
}

// TearDown runs at the end of ALL tests that use this fixture.
// Ensure the smb daemon is shutdown gracefully and appropriate cleanup occurs.
func (f *fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	f.cr = nil
	if err := unmountAllSmbMounts(ctx, f.bconn); err != nil {
		s.Error("Failed to unmount all SMB mounts: ", err)
	}
	if err := f.bconn.Close(); err != nil {
		s.Error("Failed to close background page connection: ", err)
	}
	f.bconn = nil
	if err := f.server.Stop(ctx); err != nil {
		s.Error("Failed to stop smbd: ", err)
	}
	f.server = nil
	if err := os.RemoveAll(f.tempDir); err != nil {
		s.Error("Failed to remove temporary guest share: ", err)
	}
	f.tempDir = ""
}

// Reset runs between every test that implements this fixture and ensures the
// guest directory is empty and all SMB mounts are unmounted.
func (f *fixture) Reset(ctx context.Context) error {
	if err := unmountAllSmbMounts(ctx, f.bconn); err != nil {
		return err
	}
	return removeAllContents(ctx, f.guestDir)
}

func (f *fixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// CreateGuestSambaConf creates a very simple smb.conf in the confLocation and
// ensures it has a single share visible.
// TODO(crbug.com/1156844): Make this into a fluent API to enable additional
// shares and testing of other Samba configuration.
func CreateGuestSambaConf(ctx context.Context, sharePath, confLocation string) (string, error) {
	sambaConf := `[guestshare]
	path = ` + sharePath + `
	guest ok = yes
	browseable = yes
	create mask = 0660
	directory mask = 0770
	read only = no`

	sambaFileLocation := filepath.Join(confLocation, "smb.conf")
	return sambaFileLocation, ioutil.WriteFile(sambaFileLocation, []byte(sambaConf), 0644)
}

// unmountAllSmbMounts uses the chrome.fileManagerPrivate.removeMount API to
// unmount all the identified SMB FUSE filesystems. Chrome maintains a mapping
// of SMB shares so if we unmount via cros-disks it still thinks the volume is
// mounted with chained tests all failing after the first.
func unmountAllSmbMounts(ctx context.Context, bconn *chrome.Conn) error {
	info, err := sysutil.MountInfoForPID(sysutil.SelfPID)
	if err != nil {
		return errors.Wrap(err, "failed to mount info")
	}
	for i := range info {
		if info[i].Fstype == "fuse.smbfs" {
			smbfsUniqueID := filepath.Base(info[i].MountPath)
			if err := bconn.Call(ctx, nil, "chrome.fileManagerPrivate.removeMount", "smb:"+smbfsUniqueID); err != nil {
				testing.ContextLogf(ctx, "Failed to unmount smb mountpoint %q: %v", smbfsUniqueID, err)
				continue
			}
			testing.ContextLog(ctx, "Unmounted smb mountpoint ", smbfsUniqueID)
		}
	}
	return nil
}

// removeAllContents removes all files / folders of the supplied path, but leave
// the path still available.
func removeAllContents(ctx context.Context, path string) error {
	dir, err := ioutil.ReadDir(path)
	if err != nil {
		return errors.Wrapf(err, "failed to read path %q: %v", path, err)
	}
	for _, subdir := range dir {
		subdirPath := filepath.Join(path, subdir.Name())
		if err := os.RemoveAll(subdirPath); err != nil {
			testing.ContextLogf(ctx, "Failed to remove subdirectory %q: %v", subdirPath, err)
		}
	}
	return nil
}
