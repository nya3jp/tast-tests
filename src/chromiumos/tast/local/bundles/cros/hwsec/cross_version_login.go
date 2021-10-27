// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
  "context"

  "chromiumos/tast/common/testexec"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrossVersionLogin,
		Desc: "Login test accross the version",
		Contacts: []string{
			"chingkang@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
    Params: []testing.Param {{
        Name: "betty",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("betty")),
        Val:               []string{
          "cross_version_login_data_betty_R96-14248.0.test.tar.xz",
          "cross_version_login_data_betty_R91-13904.77.0.tar.xz",
          "cross_version_login_data_betty_R91-13904.78.0.tar.xz",
        },
        ExtraData:         []string{
          "cross_version_login_data_betty_R96-14248.0.test.tar.xz",
          "cross_version_login_data_betty_R91-13904.77.0.tar.xz",
          "cross_version_login_data_betty_R91-13904.78.0.tar.xz",
        },
    }},
	})
}

func CrossVersionLogin(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	//cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

  backupPath := "/tmp/cross_version_testing_data.tar.xz"
  if err := hwseclocal.CreateCrossVersionLoginData(ctx, daemonController, backupPath); err != nil {
    s.Fatal("Failed to backup login data: ", err)
  }
  defer func() {
    if err := hwseclocal.LoadCrossVersionLoginData(ctx, daemonController, backupPath); err != nil {
      s.Fatal("Failed to load login data: ", err)
    }
    if err := testexec.CommandContext(ctx, "rm", "-f", backupPath).Run(); err != nil {
      s.Fatal("Failed to remove backup data: ", err)
    }
  }()

	fileNames := s.Param().([]string)
  for _, fn := range fileNames {
    filePath := s.DataPath(fn)
    s.Log("Testing login with: ", fn)

    if err := hwseclocal.LoadCrossVersionLoginData(ctx, daemonController, filePath); err != nil {
      s.Fatal("Failed to load login data: ", err)
    }

    // Check the functionality of cryptohome
    const (
      user = "testuser@gmail.com"
      password = "testpass"
    )
    s.Log("Testing check_key_ex")
    if err := testexec.CommandContext(ctx, "cryptohome", "--action=check_key_ex", "--user="+user, "--password="+password).Run(testexec.DumpLogOnError); err != nil {
      s.Fatal("Failed to check_key_ex: ", err)
    }
    s.Log("Testing list_key_ex")
    if err := testexec.CommandContext(ctx, "cryptohome", "--action=list_keys_ex", "--user="+user, "--password="+password).Run(testexec.DumpLogOnError); err != nil {
      s.Fatal("Failed to list_key_ex: ", err)
    }
  }
}

