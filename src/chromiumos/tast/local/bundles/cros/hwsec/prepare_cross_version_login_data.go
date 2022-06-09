// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrepareCrossVersionLoginData,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Create snapshot of login-related data, which will be used in hwsec.CrossVersionLogin to mock the login data in older version (see go/cros-cross-version-login-testing)",
		Contacts: []string{
			"chingkang@google.com",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "tpm2_simulator"},
	})
}

func PrepareCrossVersionLoginData(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// The tmpDir would not be removed at the end of test because we want to upload the data and fetch it when running hwsec.CrossVersionLogin.
	const tmpDir = "/tmp/cross_version_login"
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		s.Fatalf("Failed to create directory %q: %v", tmpDir, err)
	}

	dataPath := filepath.Join(tmpDir, "data.tar.gz")
	configPath := filepath.Join(tmpDir, "config.json")
	if err := util.PrepareCrossVersionLoginData(ctx, s.Logf, cryptohome, daemonController, dataPath, configPath); err != nil {
		s.Fatal("Failed to prepare cross-vesrion login data: ", err)
	}
}
