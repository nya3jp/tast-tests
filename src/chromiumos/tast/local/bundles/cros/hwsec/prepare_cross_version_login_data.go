// Copyright 2022 The ChromiumOS Authors
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

	// This test is only running manually to generate the login data for hwsec.CrossVersionLogin. The data would be uploaded by the scripts (See src/platform2/hwsec-test-utils/cross_version_login/prepare_cross_version_login_data.sh)..
	// Therefore, the tmpDir would not be removed at the end of test because the data would be uploaded laterand then fetched when running hwsec.CrossVersionLogin.
	const tmpDir = "/tmp/cross_version_login"
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		s.Fatalf("Failed to create directory %q: %v", tmpDir, err)
	}

	dataPath := filepath.Join(tmpDir, "data.tar.gz")
	configPath := filepath.Join(tmpDir, "config.json")
	if err := util.PrepareCrossVersionLoginData(ctx, s.Logf, cryptohome, daemonController, dataPath, configPath); err != nil {
		s.Fatal("Failed to prepare cross-version login data: ", err)
	}
}
