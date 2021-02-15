// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Install,
		Desc:         "Test installation repeatedly",
		Contacts:     []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Vars:         []string{"crostini.gaiaUsername", "crostini.gaiaPassword", "crostini.gaiaID"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraData:         []string{vm.ArtifactData(), crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Timeout:           140 * time.Minute,
			},
		},
		Pre: chrome.LoggedIn(),
	})
}

func Install(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	failTimes := 0

	var failures []string

	for n := 0; n < 500; n++ {
		s.Log("Log==============: ", n)

		if err := settings.OpenInstaller(ctx, tconn, cr); err != nil {
			failTimes = failTimes + 1
			failures = append(failures, fmt.Sprintf("%d, Error Msg: %s\n", failTimes, err))
			s.Log("Failed to launch installer: ", err)
			continue
		}
		ui := uiauto.New(tconn)
		installButton := nodewith.Name("Install").Role(role.Button)
		installCancel := nodewith.Name("Cancel").Role(role.Button)
		if err := uiauto.Combine("click install and wait it to finish",
			ui.LeftClickUntil(installButton, ui.WithTimeout(2*time.Second).WaitUntilExists(installCancel)))(ctx); err != nil {
			failTimes = failTimes + 1
			failures = append(failures, fmt.Sprintf("%d, Error Msg: %s\n", failTimes, err))
			s.Log("Failed to click button Install: ", err)
			continue
		}
		s.Log("Wait to click Cancel")
		testing.Sleep(5 * time.Second)
		ui.LeftClick(installCancel)(ctx)
		ui.WithTimeout(time.Minute).WaitUntilGone(nodewith.NameRegex(regexp.MustCompile(`^Set up Linux`)).Role(role.RootWebArea))(ctx)
	}
	s.Log("Results======================")
	s.Log("Failed times: ", failTimes)
	s.Log(failures)
	s.Log("Results======================")
}
