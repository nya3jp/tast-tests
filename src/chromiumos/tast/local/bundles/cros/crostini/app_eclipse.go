// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppEclipse,
		Desc:         "Test Eclipse in Terminal window",
		Contacts:     []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"keepState"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		SoftwareDeps: []string{"chrome", "vm_host", "amd64"},
		Params: []testing.Param{
			// Parameters generated by params_test.go. DO NOT EDIT.
			{
				ExtraData:         []string{crostini.GetContainerMetadataArtifact("buster", true), crostini.GetContainerRootfsArtifact("buster", true)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniAppTest,
				Pre:               crostini.StartedByDlcBusterLargeContainer(),
				Timeout:           15 * time.Minute,
			},
		},
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
	defer crostini.RunCrostiniPostTest(cleanupCtx, s.PreValue().(crostini.PreData))

	// Open Terminal app.
	terminalApp, err := terminalapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Terminal app: ", err)
	}

	defer func() {
		// Restart Crostini in the end because it is not possible to control the Crostini app.
		// TODO(jinrongwu): modify this once it is possible to control Eclipse.
		if err := terminalApp.RestartCrostini(keyboard, cont, cr.NormalizedUser())(cleanupCtx); err != nil {
			s.Log("Failed to restart Crostini: ", err)
		}
	}()

	// Create a workspace and a test file.
	const (
		workspace = "ws"
		testFile  = "test.java"
	)
	if err := cont.Command(ctx, "mkdir", workspace).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to create workspace directory in the Container")
	}
	if err := cont.Command(ctx, "touch", fmt.Sprintf("%s/%s", workspace, testFile)).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to create test file in the Container: ", err)
	}

	// Find eclipse window.
	name := fmt.Sprintf("%s - /home/%s/%s/%s - Eclipse IDE ", workspace, strings.Split(cr.NormalizedUser(), "@")[0], workspace, testFile)
	eclipseWindow := nodewith.Name(name).Role(role.Window).First()
	if err := uiauto.Combine("start Eclipse",
		terminalApp.RunCommand(keyboard, fmt.Sprintf("eclipse -data %s --launcher.openFile %s/%s --noSplash", workspace, workspace, testFile)),
		uiauto.New(tconn).WaitUntilExists(eclipseWindow),
		crostini.TakeAppScreenshot("eclipse"))(ctx); err != nil {
		s.Fatal("Failed to start Eclipse in Terminal: ", err)
	}

	//TODO(jinrongwu): UI test on eclipse.
}
