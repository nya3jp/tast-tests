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
		Func:         MountVaultFailure,
		Desc:         "Verify the error from login failure is properly recorded",
		Contacts:     []string{"zuan@google.com", "cros-hwsecy@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{},
		Params: []testing.Param{{
			Name: "normal_mount",
			Val:  "normal_mount",
		}, {
			Name: "non_existent",
			Val:  "non_existent",
		}, {
			Name: "wrong_password",
			Val:  "wrong_password",
		}, {
			Name: "actual_error",
			Val:  "actual_error",
		}},
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

func MountVaultFailure(ctx context.Context, s *testing.State) {
	// Phase 1: Setup for crash-related services.

	// SetUpCrashTest will create a file filter to ignore core dumps whose names don't match
	// the pattern in FilterCrashes. So clean it up after the TearDownCrashTest

	opt := crash.WithMockConsent()
	if err := crash.SetUpCrashTest(ctx, crash.FilterCrashes("cryptohome"), opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	if err := crash.RestartAnomalyDetector(ctx); err != nil {
		s.Fatal("Failed to restart anomaly detector: ", err)
	}

	// Use system crash dir because it only happens without login.
	crashDirs := []string{crash.SystemCrashDir}

	// Phase 2: Setup cryptohome related stuff.
	cmdRunner := hwseclocal.NewCmdRunner()

	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	cryptohome := helper.CryptohomeClient()

	// Cleanup to ensure system is in a consistent state
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*5)
	defer cancel()
	defer func(ctx context.Context) {
		if _, err := cryptohome.Unmount(ctx, loginUsername); err != nil {
			s.Error("Failed to unmount vault: ", err)
		}
		if _, err := cryptohome.RemoveVault(ctx, loginUsername); err != nil {
			s.Fatal("Failed to remove vault: ", err)
		}
	}(ctxForCleanup)

	testType := s.Param().(string)

	// Do we expect report for the current test?
	expectReport := false

	// Phase 2: Run our situation.
	if testType == "normal_mount" {
		// Then try mounting a normal user, should not generate report.
		if err := cryptohome.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), true, hwsec.NewVaultConfig()); err != nil {
			s.Fatal("Fail to mount normally: ", err)
		}
		cryptohome.UnmountAll(ctx)

		expectReport = false
	} else if testType == "non_existent" {
		// A failure to mount due to non-existent user, should not generate report.
		if err := cryptohome.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), false, hwsec.NewVaultConfig()); err == nil {
			s.Fatal("Mounting non-existent user should fail")
		}

		expectReport = false
	} else if testType == "wrong_password" {
		// We need to create the account first.
		if err := cryptohome.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), true, hwsec.NewVaultConfig()); err != nil {
			s.Fatal("Fail to mount normally: ", err)
		}
		cryptohome.UnmountAll(ctx)

		if err := cryptohome.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginWrongPassword), false, hwsec.NewVaultConfig()); err == nil {
			s.Fatal("Mounting with incorrect password succeeded")
		}

		expectReport = false
	} else if testType == "actual_error" {
		// We need to create the account first.
		if err := cryptohome.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), true, hwsec.NewVaultConfig()); err != nil {
			s.Fatal("Fail to mount normally: ", err)
		}
		cryptohome.UnmountAll(ctx)

		hash, err := cryptohome.GetSanitizedUsername(ctx, loginUsername, false)
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
		if err := cryptohome.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), false, hwsec.NewVaultConfig()); err == nil {
			s.Fatal("Mounting corrupted vault should fail")
		}

		expectReport = true
	} else {
		s.Fatalf("Unknown test type: %s", testType)
	}

	// Phase 3: Verify if report is generated.

	s.Log("Waiting for files")
	_, waitForCrashErr := crash.WaitForCrashFiles(ctx, crashDirs, loginExpectedRegexes, crash.Timeout(loginCrashTimeout*time.Second))
	if expectReport {
		// Report is expected.
		if waitForCrashErr != nil {
			s.Error("Crash not found for actual login issue: ", waitForCrashErr)
		}
	} else {
		// Report is not expected.
		if waitForCrashErr == nil {
			s.Error("Crash found when it's WAI")
		}
	}
}
