// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SMB,
		Desc: "Mount and check a file on Samba SMB share",
		Contacts: []string{
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Timeout:      7 * time.Minute,
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Data:         []string{"smb.conf", crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		HardwareDeps: crostini.CrostiniStable,
	})
}

func SMB(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	tconn := pre.TestAPIConn
	cont := pre.Container
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	const smbConfigFile = "smb.conf"
	if err := cont.PushFile(ctx, s.DataPath(smbConfigFile), "/tmp/smb.conf"); err != nil {
		s.Fatal("Failed copying smb.conf into container: ", err)
	}

	if err := setupSambaShare(ctx, cont); err != nil {
		s.Fatal("Failed executing Samba step commands: ", err)
	}

	// Launch the files application
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	defer files.Root.Release(ctx)

	menuItems := []string{"Add new service", "SMB file share"}
	if err := files.ClickMoreMenuItem(ctx, menuItems); err != nil {
		s.Fatal("Failed clicking menu item SMB file share: ", err)
	}

	// Get SMB Window that has just popped up
	params := ui.FindParams{
		Name: "Add file share",
		Role: ui.RoleTypeDialog,
	}

	smbWindow, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the newly launched smb window: ", err)
	}
	defer smbWindow.Release(ctx)

	// Click the SMB file share input box to enter details
	params = ui.FindParams{
		Name: `\servershare`,
		Role: ui.RoleTypeStaticText,
	}
	fileShareURLTextBox, err := smbWindow.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed waiting for file share url text box: ", err)
	}
	defer fileShareURLTextBox.Release(ctx)

	if err := fileShareURLTextBox.LeftClick(ctx); err != nil {
		s.Fatal("Failed clicking on the file share url text box: ", err)
	}

	// Get a handle to the input keyboard
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	if err := kb.Type(ctx, `\\penguin.linux.test\guestshare`); err != nil {
		s.Fatal("Failed entering the new SMB file share path: ", err)
	}

	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed pressing enter: ", err)
	}

	// Click freshly loaded SMB share to open the folder.
	params = ui.FindParams{
		Name: "guestshare",
		Role: ui.RoleTypeTreeItem,
	}
	smbshare, err := files.Root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed finding the SMB file share in the directory tree: ", err)
	}
	defer smbshare.Release(ctx)
	if err := smbshare.LeftClick(ctx); err != nil {
		s.Fatal("Failed clicking on the SMB File share item: ", err)
	}

	// Verify the file shows up in list view
	if err := files.WaitForFile(ctx, "test.txt", 10*time.Second); err != nil {
		s.Fatal("Failed waiting for test file: ", err)
	}
}

func setupSambaShare(ctx context.Context, cont *vm.Container) error {
	setupCommands := []string{
		"echo 'samba-common samba-common/workgroup string WORKGROUP' | sudo debconf-set-selections",
		"echo 'samba-common samba-common/dhcp boolean false' | sudo debconf-set-selections",
		"echo 'samba-common samba-common/do_debconf boolean false' | sudo debconf-set-selections",
		"sudo apt -y install samba",
		"sudo mkdir -p /pub/guestshare",
		"echo 'test file contents' | sudo tee /pub/guestshare/test.txt",
		"sudo chown -R nobody:nogroup /pub/guestshare",
		"sudo chmod 0770 /pub/guestshare",
		"sudo useradd -d /home/smbuser -m smbuser",
		"echo 'password' > sudo smbpasswd -s -a smbuser",
		"sudo cp -f /tmp/smb.conf /etc/samba/",
		"sudo service smbd start",
	}

	for _, cmd := range setupCommands {
		if err := cont.Command(ctx, "sh", "-c", cmd).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed command: %s", cmd)
		}
	}

	return nil
}
