// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crash"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LoginFailure,
		Desc:         "Verify the error from login failure is properly recorded",
		Contacts:     []string{"zuan@google.com", "cros-hwsecy@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{},
	})
}

const (
	loginCrashBaseName = `mount_failure_cryptohome\.\d{8}\.\d{6}\.\d+\.0`
	loginCrashTimeout  = 60

	// Example user information
	loginUsername      = "fakeuser1@example.com"
	loginPassword      = "FakePasswordForFakeUser1"
	loginWrongPassword = "Incorrect"
	loginLabel         = "PasswordLabel"
)

var (
	loginExpectedRegexes = []string{loginCrashBaseName + `\.log`, loginCrashBaseName + `\.meta`}
)

func LoginFailure(ctx context.Context, s *testing.State) {

	// Phase 1: Setup for crash-related services.

	// SetUpCrashTest will create a file filter to ignore core dumps whose names don't match
	// the pattern in FilterCrashes. So clean it up after the TearDownCrashTest
	defer crash.CleanupDevcoredump(ctx)

	opt := crash.WithMockConsent()
	if err := crash.SetUpCrashTest(ctx, crash.FilterCrashes("cryptohome"), opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	if err := crash.RestartAnomalyDetectorWithSendAll(ctx, true); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}
	// Restart anomaly detector to clear its --testonly-send-all flag at the end of execution.
	defer crash.RestartAnomalyDetector(ctx)

	crashDirs, err := crash.GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		s.Fatal("Couldn't get daemon store dirs: ", err)
	}
	// We might not be logged in, so also allow system crash dir.
	crashDirs = append(crashDirs, crash.SystemCrashDir)

	// Phase 2: Setup cryptohome related stuff.
	cmdRunner := hwseclocal.NewCmdRunner()

	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	utility := helper.CryptohomeClient()

	// Cleanup to ensure system is in a consistent state
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*5)
	defer cancel()
	defer func(ctx context.Context) {
		if _, err := utility.Unmount(ctx, loginUsername); err != nil {
			s.Error("Failed to unmount vault: ", err)
		}
		if _, err := utility.RemoveVault(ctx, loginUsername); err != nil {
			s.Fatal("Failed to remove vault: ", err)
		}
	}(ctxForCleanup)

	// Phase 2: Check that WAI situation is not reported.

	// First try a normal mount that failed due to non-existent user.
	if err := utility.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), false, hwsec.NewVaultConfig()); err == nil {
		s.Fatal("Mounting non-existent user should fail")
	}

	// Then try mounting a normal user.
	if err := utility.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Fail to mount normally: ", err)
	}
	utility.UnmountAll(ctx)

	// Then try to mount with an incorrect password.
	if err := utility.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginWrongPassword), false, hwsec.NewVaultConfig()); err == nil {
		s.Fatal("Mounting with incorrect password succeeded")
	}

	// Make sure nothing is recorded.
	s.Log("Waiting for files")
	if _, err := crash.WaitForCrashFiles(ctx, crashDirs, loginExpectedRegexes, crash.Timeout(loginCrashTimeout*time.Second)); err == nil {
		s.Fatal("Crash found when it's WAI")
	}

	// Phase 3: Try to make the vault unmountable to cause a failure that should be reported.
	hash, err := utility.GetSanitizedUsername(ctx, loginUsername, false)
	if err != nil {
		s.Fatal("Failed to get sanitized username: ", err)
	}

	userDir := fmt.Sprintf("/home/user/%s", hash)
	if _, err := cmdRunner.Run(ctx, "rm", "-rf", userDir); err != nil {
		s.Fatal("Failed to remove user diretory: ", err)
	}
	if _, err := cmdRunner.Run(ctx, "touch", userDir); err != nil {
		s.Fatal("Failed to make user directory a file: ", err)
	}

	// Try the mount again, it should fail.
	if err := utility.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), false, hwsec.NewVaultConfig()); err == nil {
		s.Fatal("Mounting corrupted vault should fail")
	}

	// Crash should be recorded.
	s.Log("Waiting for files")
	if _, err := crash.WaitForCrashFiles(ctx, crashDirs, loginExpectedRegexes, crash.Timeout(loginCrashTimeout*time.Second)); err != nil {
		s.Fatal("Crash not found for actual login issue: ", err)
	}
}
