// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AppEclipse,
		Desc:     "Test eclipse in Terminal window",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:    "download_buster",
			Pre:     crostini.StartedByDownloadBuster(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host", "amd64"},
	})
}
func AppEclipse(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cr := s.PreValue().(crostini.PreData).Chrome
	keyboard := s.PreValue().(crostini.PreData).Keyboard
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 90*time.Second)
	defer cancel()

	userName := strings.Split(cr.User(), "@")[0]

	// Open Terminal app.
	terminalApp, err := terminalapp.Launch(cleanupCtx, tconn, userName)
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}

	// Restart crostini in the end in case any error in the middle and eclipse is not closed.
	// This also closes the Terminal window.
	defer func() {
		if err := terminalApp.RestartCrostini(ctx, keyboard, cont, cr.User()); err != nil {
			s.Fatal("Failed to restart crostini: ", err)
		}
	}()

	// Install eclipse in container. This is a work around until eclipse is pre-installed in a image.
	if err := installEclipseInContainer(ctx, cont, userName); err != nil {
		s.Fatal("Failed to install eclipse in container: ", err)
	}

	// Create a workspace and a test file.
	const (
		workspace = "ws"
		testFile  = "test.java"
	)
	if err := cont.RunMultiCommandsInSequence(ctx, fmt.Sprintf("mkdir %s", workspace), fmt.Sprintf("touch %s/%s", workspace, testFile)); err != nil {
		s.Fatal("Failed to create test file in the container: ", err)
	}

	// Open eclipse.
	if err := terminalApp.RunCommand(ctx, keyboard, fmt.Sprintf("sommelier -X --scale=0.5 eclipse -data %s --launcher.openFile %s/%s --noSplash", workspace, workspace, testFile)); err != nil {
		s.Fatal("Failed to start eclipse in Terminal: ", err)
	}

	// Find eclipse window.
	param := ui.FindParams{
		Name: fmt.Sprintf("%s - /home/%s/%s/%s - Eclipse IDE ", workspace, userName, workspace, testFile),
		Role: ui.RoleTypeWindow,
	}
	if _, err := ui.FindWithTimeout(ctx, tconn, param, 90*time.Second); err != nil {
		s.Fatal("Failed to find eclipse window: ", err)
	}

	//TODO(jinrongwu): UI test on eclipse.
}

// installEclipseInContainer installs eclipse in container.
func installEclipseInContainer(ctx context.Context, cont *vm.Container, userName string) error {
	// Check whether eclipse has been installed or not.
	if err := cont.CheckFilesExistInDir(ctx, "/usr/bin", "eclipse"); err == nil {
		return nil
	}

	testing.ContextLog(ctx, "Installing eclipse")
	downloadFile := "eclipse.tar.gz"
	if err := cont.RunMultiCommandsInSequence(ctx, "sudo apt-get update",
		"sudo DEBIAN_FRONTEND=noninteractive apt-get -y install default-jre",
		"wget https://storage.googleapis.com/chromiumos-test-assets-public/crostini_test_files/eclipse.tar.gz",
		fmt.Sprintf("sudo tar -zxvf %s -C /usr/", downloadFile),
		"sudo ln -s /usr/eclipse/eclipse /usr/bin/eclipse",
		fmt.Sprintf("sudo rm %s", downloadFile)); err != nil {
		return errors.Wrap(err, "failed to install eclipse in container")
	}

	// Create eclipse launcher file.
	launcherFile := "eclipse.desktop"
	localPath := filepath.Join(filesapp.DownloadPath, launcherFile)
	fileContent := "[Desktop Entry]\nEncoding=UTF-8\nName=Eclipse IDE\nComment=Eclipse IDE\nExec=/usr/bin/eclipse\nIcon=/usr/eclipse/icon.xpm\nCategories=Application;Development;Java;IDE\nType=Application\nTerminal=0"
	if err := ioutil.WriteFile(localPath, []byte(fileContent), 0644); err != nil {
		return errors.Wrapf(err, "failed to create %s in Downloads: ", localPath)
	}

	destinationPath := "/usr/share/applications/eclipse.desktop"
	tmpPath := fmt.Sprintf("/home/%s/%s", userName, launcherFile)
	if err := cont.PushFile(ctx, localPath, tmpPath); err != nil {
		return errors.Wrapf(err, "failed to put %s in chrome to %s in container", localPath, tmpPath)
	}

	if err := cont.Command(ctx, "sudo", "mv", tmpPath, destinationPath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to move %s to %s", tmpPath, destinationPath)
	}

	return nil
}
