// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
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
			Val: mountVaultFailureTestVal{
				action:       testNormalMount,
				expectReport: false,
			},
		}, {
			Name: "non_existent",
			Val: mountVaultFailureTestVal{
				action:       testNonExistent,
				expectReport: false,
			},
		}, {
			Name: "wrong_password",
			Val: mountVaultFailureTestVal{
				action:       testWrongPassword,
				expectReport: false,
			},
		},
		/* Disabled due to <1% pass rate over 30 days. See b/241943008
		{
			Name: "actual_error",
			Val: mountVaultFailureTestVal{
				action:       testActualError,
				expectReport: true,
			},
		}
		*/
		},
	})
}

const (
	loginCrashBaseName = `mount_failure_cryptohome\.\d{8}\.\d{6}\.\d+\.0`
	loginCrashMetaName = loginCrashBaseName + `\.meta`
	loginCrashLogName  = loginCrashBaseName + `\.log`
	loginCrashTimeout  = 60 * time.Second

	// Example user information
	loginUsername      = "fakeuser1@example.com"
	loginPassword      = "FakePasswordForFakeUser1"
	loginWrongPassword = "Incorrect"
	loginLabel         = "PasswordLabel"
)

// testAction is the type of test we're doing.
type testAction int64

// Below are the possible test actions (enum).
const (
	testNormalMount = iota
	testNonExistent
	testWrongPassword
	testActualError
)

type mountVaultFailureTestVal struct {
	// action is the type of the test.
	action testAction

	// expectReport is whether we expect report.
	expectReport bool
}

var (
	loginExpectedRegexes = []string{loginCrashLogName, loginCrashMetaName}
)

func MountVaultFailure(ctx context.Context, s *testing.State) {
	// Phase 1: Setup for crash-related services.

	// Setup cleanup context
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*5)
	defer cancel()

	// SetUpCrashTest will create a file filter to ignore core dumps whose names don't match
	// the pattern in FilterCrashes. So clean it up after the TearDownCrashTest

	opt := crash.WithMockConsent()
	if err := crash.SetUpCrashTest(ctx, crash.FilterCrashes("cryptohome"), opt); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctxForCleanup)

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
	defer func(ctx context.Context) {
		if _, err := cryptohome.Unmount(ctx, loginUsername); err != nil {
			s.Error("Failed to unmount vault: ", err)
		}
		if _, err := cryptohome.RemoveVault(ctx, loginUsername); err != nil {
			s.Fatal("Failed to remove vault: ", err)
		}
	}(ctxForCleanup)

	testParams := s.Param().(mountVaultFailureTestVal)
	testType := testParams.action
	// Do we expect report for the current test?
	expectReport := testParams.expectReport

	// Phase 3: Run our situation.
	if testType == testNormalMount {
		// Then try mounting a normal user, should not generate report.
		if err := cryptohome.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), true, hwsec.NewVaultConfig()); err != nil {
			s.Fatal("Fail to mount normally: ", err)
		}
		cryptohome.UnmountAll(ctx)
	} else if testType == testNonExistent {
		// A failure to mount due to non-existent user, should not generate report.
		if err := cryptohome.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), false, hwsec.NewVaultConfig()); err == nil {
			s.Fatal("Mounting non-existent user should fail")
		}
	} else if testType == testWrongPassword {
		// We need to create the account first.
		if err := cryptohome.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), true, hwsec.NewVaultConfig()); err != nil {
			s.Fatal("Fail to mount normally: ", err)
		}
		cryptohome.UnmountAll(ctx)

		if err := cryptohome.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginWrongPassword), false, hwsec.NewVaultConfig()); err == nil {
			s.Fatal("Mounting with incorrect password succeeded")
		}
	} else if testType == testActualError {
		// We need to create the account first.
		if err := cryptohome.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), true, hwsec.NewVaultConfig()); err != nil {
			s.Fatal("Fail to mount normally: ", err)
		}
		cryptohome.UnmountAll(ctx)

		hash, err := cryptohome.GetSanitizedUsername(ctx, loginUsername, false)
		if err != nil {
			s.Fatal("Failed to get sanitized username: ", err)
		}

		userDir := filepath.Join("/home/user", hash)
		if err := os.RemoveAll(userDir); err != nil {
			s.Fatal("Failed to remove user diretory: ", err)
		}
		if err := ioutil.WriteFile(userDir, nil, 0644); err != nil {
			s.Fatal("Failed to make user directory a file: ", err)
		}

		// Try the mount again, it should fail.
		if err := cryptohome.MountVault(ctx, loginLabel, hwsec.NewPassAuthConfig(loginUsername, loginPassword), false, hwsec.NewVaultConfig()); err == nil {
			s.Fatal("Mounting corrupted vault should fail")
		}
	} else {
		s.Fatalf("Unknown test type: %d", testType)
	}

	// Phase 4: Verify if report is generated.

	s.Log("Waiting for files")
	files, waitForCrashErr := crash.WaitForCrashFiles(ctx, crashDirs, loginExpectedRegexes, crash.Timeout(loginCrashTimeout))
	if expectReport {
		// Report is expected.
		if waitForCrashErr != nil {
			s.Error("Crash not found for actual login issue: ", waitForCrashErr)
		}
	} else {
		// Report is not expected.
		if waitForCrashErr == nil {
			if err := crash.MoveFilesToOut(ctx, s.OutDir(), files[loginCrashMetaName][0]); err != nil {
				s.Error("Failed to save unexpected crashes meta file: ", err)
			}
			if err := crash.MoveFilesToOut(ctx, s.OutDir(), files[loginCrashLogName][0]); err != nil {
				s.Error("Failed to save unexpected crashes log file: ", err)
			}
			// Note that the exact situation depends on the test parameters, i.e. testParams.action.
			s.Error("Crash found for a situation that is not supposed to generate a crash")
		}
	}
}
