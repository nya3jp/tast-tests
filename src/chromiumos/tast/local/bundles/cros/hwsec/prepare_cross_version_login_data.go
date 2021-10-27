// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PrepareCrossVersionLoginData,
		Desc: "Create data for cross-version login test",
		Contacts: []string{
			"chingkang@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func PrepareCrossVersionLoginData(ctx context.Context, s *testing.State) {
	s.Log("Restarting ui job")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	func() {
		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Failed to log in by Chrome: ", err)
		}
		defer cr.Close(ctx)
	}()
	defer cryptohome.RemoveVault(ctx, user)

	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
    s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

  dataPath := "/tmp/cross_version_testing_data.tar.xz"
  if err := hwseclocal.CreateCrossVersionLoginData(ctx, daemonController, dataPath); err != nil {
    s.Fatal("Failed to create cross version login data: ", err)
  }

	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}
}
