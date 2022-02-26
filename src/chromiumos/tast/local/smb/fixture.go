// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package smb

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	smbdSetupTimeout = 5 * time.Second
	smbpasswdFile    = "smbpasswd"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "smbStarted",
		Desc:            "Samba server started with 2 shares available",
		Contacts:        []string{"chromeos-files-syd@chromium.org", "benreich@chromium.org"},
		Parent:          "chromeLoggedIn",
		Impl:            &fixture{},
		SetUpTimeout:    chrome.LoginTimeout + smbdSetupTimeout,
		ResetTimeout:    smbdSetupTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{smbpasswdFile},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "smbStartedWithoutChrome",
		Desc: `Samba server started with 2 shares available with
						  unmounting of SMB mounts and Chrome cleanup expected
						  to be handled by the test`,
		Contacts:        []string{"chromeos-files-syd@chromium.org", "benreich@chromium.org"},
		Impl:            &fixture{startChrome: false},
		SetUpTimeout:    smbdSetupTimeout,
		ResetTimeout:    smbdSetupTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{smbpasswdFile},
	})
}

// FixtureData is the struct exposed to tests.
type FixtureData struct {
	Chrome         *chrome.Chrome
	Server         *Server
	GuestSharePath string
}

type fixture struct {
	// True starts a Chrome instance within this fixture, used if Chrome needs
	// needs to be restarted or manipulated as it doesn't get locked.
	startChrome bool

	cr       *chrome.Chrome
	server   *Server
	guestDir string
	tempDir  string
}

// SetUp starts a smbd daemon, sets up a temporary directory that contains a
// minimal samba guest configuration and a folder for a public SMB share.
func (f *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	success := false
	if f.startChrome {
		f.cr = s.ParentValue().(*chrome.Chrome)
	}
	defer func() {
		if !success {
			f.cr = nil
		}
	}()

	// Create a temporary directory for smb.conf and a guestshare, this should be
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

	f.guestDir = filepath.Join(dir, "guestshare")
	if err := os.MkdirAll(f.guestDir, 0777); err != nil {
		s.Fatal("Failed to create guestshare: ", err)
	}

	// As we're forcing the user to chronos the guestshare directory must be
	// owned to ensure files can be copied across.
	if err := os.Chown(f.guestDir, 1000, 1000); err != nil {
		s.Fatal("Failed to chown guestshare to chronos: ", err)
	}

	guestSambaConf, err := createGuestSambaConf(ctx, f.guestDir, dir)
	if err != nil {
		s.Fatal("Failed to create guest samba configuration: ", err)
	}

	// Move the passdb file to Samba temporary directory.
	if err := fsutil.CopyFile(s.DataPath("smbpasswd"), filepath.Join(f.tempDir, smbpasswdFile)); err != nil {
		s.Fatal("Failed to copy smbpasswd file: ", err)
	}

	// Start the smbd process which will dump an error log if it is not
	// terminated by a SIGTERM.
	server := NewServer(guestSambaConf)
	if err := server.Start(ctx); err != nil {
		s.Fatal("Failed to start smbd daemon: ", err)
	}
	f.server = server

	success = true
	return FixtureData{
		Chrome:         f.cr,
		Server:         server,
		GuestSharePath: f.guestDir,
	}
}

// TearDown ensures the smb daemon is shutdown gracefully and all the temporary
// directories and files are cleaned up.
func (f *fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.startChrome && f.cr != nil {
		if err := UnmountAllSmbMounts(ctx, f.cr); err != nil {
			s.Error("Failed to unmount all SMB mounts: ", err)
		}
	}
	f.cr = nil
	if err := f.server.Stop(ctx); err != nil {
		s.Error("Failed to stop smbd: ", err)
	}
	f.server = nil
	if err := os.RemoveAll(f.tempDir); err != nil {
		s.Error("Failed to remove temporary guest share: ", err)
	}
	f.tempDir = ""
}

// Reset unmounts any mounted SMB shares and removes all the contents of the
// guest share in between tests.
func (f *fixture) Reset(ctx context.Context) error {
	if f.startChrome && f.cr != nil {
		if err := UnmountAllSmbMounts(ctx, f.cr); err != nil {
			testing.ContextLog(ctx, "Failed to unmount all SMB mounts: ", err)
		}
	}
	return removeAllContents(ctx, f.guestDir)
}

func (f *fixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// createGuestSambaConf creates a very simple smb.conf in the confLocation and
// ensures it has a single share visible.
// TODO(crbug.com/1156844): Make this into a fluent API to enable additional
// shares and testing of other Samba configuration.
func createGuestSambaConf(ctx context.Context, sharePath, confLocation string) (string, error) {
	sambaConf := `private dir = ` + confLocation + `
[global]
	security = user
	smb passwd file = ` + filepath.Join(confLocation, smbpasswdFile) + `
	passdb backend = smbpasswd

[guestshare]
	path = ` + sharePath + `
	guest ok = yes
	writeable = yes
	browseable = yes
	create mask = 0644
	directory mask = 0755
	force user = chronos
	read only = no

[secureshare]
	path = ` + sharePath + `
	guest ok = no
	writeable = yes
	browseable = yes
	create mask = 0644
	directory mask = 0755
	valid users = chronos
	read only = no`

	sambaFileLocation := filepath.Join(confLocation, "smb.conf")
	return sambaFileLocation, ioutil.WriteFile(sambaFileLocation, []byte(sambaConf), 0644)
}

// UnmountAllSmbMounts uses the chrome.fileManagerPrivate.removeMount API to
// unmount all the identified SMB FUSE filesystems. Chrome maintains a mapping
// of SMB shares so if we unmount via cros-disks it still thinks the volume is
// mounted with chained tests all failing after the first.
func UnmountAllSmbMounts(ctx context.Context, cr *chrome.Chrome) error {
	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create the test API conn")
	}
	// Open the Files App.
	if _, err := filesapp.Launch(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to launch Files app")
	}
	// Get connection to foreground Files app to verify changes.
	filesChromeApp := "chrome-extension://" + apps.Files.ID + "/main.html"
	filesSWA := "chrome://file-manager/"
	matchFilesApp := func(t *chrome.Target) bool {
		return t.URL == filesChromeApp || strings.HasPrefix(t.URL, filesSWA)
	}
	conn, err := cr.NewConnForTarget(ctx, matchFilesApp)
	if err != nil {
		return errors.Wrap(err, "failed to connect to Files app foreground window")
	}
	defer conn.Close()

	info, err := sysutil.MountInfoForPID(sysutil.SelfPID)
	if err != nil {
		return errors.Wrap(err, "failed to mount info")
	}
	for i := range info {
		if info[i].Fstype == "fuse.smbfs" {
			smbfsUniqueID := filepath.Base(info[i].MountPath)
			if err := conn.Call(ctx, nil, "chrome.fileManagerPrivate.removeMount", "smb:"+smbfsUniqueID); err != nil {
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
