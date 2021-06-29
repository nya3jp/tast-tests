// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// NOTE: This test is somewhat similar to hwsec.DictionaryAttackLockoutResetTPM2 (a local test), if change is
// made to one, it is likely that the other have to be changed as well.
// The referred test is specifically for TPMv2.0, while this test is for TPMv1.2.
// Both versions of TPM is incompatible with each other and the available NVRAM index differs across the 2 versions.
// Therefore, we need 2 versions of the test.
// This version uses existing NVRAM space (endorsement cert) on TPMv1.2. Reading it with incorrect auth on
// TPMv1.2 will generate dictionary attack counter increment needed by this test. However, on TPMv2.0, this
// behaviour is different so the same method is not used in the other test.

func init() {
	testing.AddTest(&testing.Test{
		Func: DictionaryAttackLockoutResetTPM1,
		Desc: "Verifies that for TPMv1.2 devices, dictionary attack counter functions correctly and can be reset",
		Contacts: []string{
			"zuan@chromium.org",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"tpm1"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// DictionaryAttackLockoutResetTPM1 checks that get dictionary attack info and reset dictionary attack lockout works as expected.
func DictionaryAttackLockoutResetTPM1(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	tpmManager := helper.TPMManagerClient()

	// In this test, we want to check if DA counter will be reset when it increases.
	// Check DA Counter => Read NVRAM Index with incorrect password => Check DA Counter
	// Read NVRAM Index with incorrect password is used to trigger an increase in DA counter.

	// Check if the DA is not locked out before we increase the DA counter.
	err = hwseclocal.CheckDAIsZero(ctx, tpmManager)
	if err != nil {
		s.Fatal("Failed to check DA counter is zero: ", err)
	}

	err = hwseclocal.IncreaseDAForTpm1(ctx, tpmManager)
	if err != nil {
		s.Fatal("Failed to increase dictionary attcack counter: ", err)
	}

	// Check if the DA counter is reset properly.
	err = hwseclocal.CheckDAIsZero(ctx, tpmManager)
	if err != nil {
		s.Fatal("Failed to check DA counter is zero: ", err)
	}

	logReader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("Failed to create log reader: ", err)
	}

	// restart tcsd to generate auth failure log
	_, err = testexec.CommandContext(ctx, "restart", "tcsd").Output()
	if err != nil {
		s.Fatal("Failed to restart tcsd: ", err)
	}
	// Restart tpm_managerd to avoid tpm_managerd crashing when receiving next command, see b/192034446.
	// TODO(b/192034446): remove this once the problem is resolved.
	_, err = testexec.CommandContext(ctx, "restart", "tpm_managerd").Output()
	if err != nil {
		s.Fatal("Failed to restart tpm_managerd: ", err)
	}

	// Sleep a while to ensure that log has beed generated.
	testing.Sleep(ctx, 5*time.Second)

	authFailureRegexp := regexp.MustCompile(`Found auth failure in the last life cycle. \(0x.*\)`)
	anomalyRegexp := regexp.MustCompile(`(anomaly_detector invoking crash_reporter with --auth_failure)|(Ignoring auth_failure 0x.*)`)
	foundAuthFailure := false
	foundAnomaly := false
	for {
		entry, err := logReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.Fatal("Failed to read log: ", err)
		}
		if authFailureRegexp.Match([]byte(entry.Content)) {
			foundAuthFailure = true
		}
		if anomalyRegexp.Match([]byte(entry.Content)) {
			foundAnomaly = true
		}
	}
	if !foundAuthFailure {
		s.Fatalf("Failed to find auth_failure in %s", syslog.MessageFile)
	}
	if !foundAnomaly {
		s.Fatal("Failed to trigger anomaly detector")
	}
}
