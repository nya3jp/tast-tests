// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

// deviceTargetVersionPrefix contains the test parameters which are different
// between fake DMS and real DPanel server.
type deviceTargetVersionPrefix struct {
	// True if using fake DMS. False if using real DPanel server.
	fakeDMS bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceTargetVersionPrefix,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that target version prefix policy is respected in auto update",
		Contacts: []string{
			"yixie@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromeEnrolledLoggedIn,
		// TODO(b/230317245): Add variant with real DPanel server after go/tape-tast
		//                    is implemented and available.
		Params: []testing.Param{
			{
				Name: "fake_dms",
				Val: deviceTargetVersionPrefix{
					fakeDMS: true,
				},
			},
		},
	})
}

const (
	etcLSBPath            = "/etc/lsb-release"
	statefulLSBPath       = "/mnt/stateful_partition/etc/lsb-release"
	targetVersionInPolicy = "14150.*"

	// Fake OS version R94-14150.87.0, which is the last release of R94.
	fakeOSVersion = "14150.87.0"

	updateCheckStarted    = "Initiating update check."
	updateStatusIdle      = "UPDATE_STATUS_IDLE"
	updateStatusChecking  = "UPDATE_STATUS_CHECKING_FOR_UPDATE"
	updateStatusAvailable = "UPDATE_STATUS_UPDATE_AVAILABLE"
	updateErrorNoUpdate   = "ErrorCode::kNoUpdate"
)

func DeviceTargetVersionPrefix(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Make a backup of lsb-release in stateful partition
	statefulLSB, _ := lsbrelease.LoadFrom(statefulLSBPath)

	// Load lsb-release template from /etc/lsb-release.
	etcLSB, err := lsbrelease.LoadFrom(etcLSBPath)
	if err != nil {
		s.Fatal("Failed to load original "+etcLSBPath, err)
	}

	// Generate signed board name to receive updates
	board, ok := etcLSB[lsbrelease.Board]
	if !ok {
		s.Fatal("Failed to get board from "+etcLSBPath, err)
	}
	signedBoard := board + "-signed-mp-v2keys"
	etcLSB[lsbrelease.Board] = signedBoard

	// Modify OS version
	etcLSB[lsbrelease.Version] = fakeOSVersion

	// Values in stateful lsb-release overrides /etc/lsb-release.
	testing.ContextLog(ctx, "Overwriting OS version in stateful lsb-release")
	if err := writeStatefulLSB(etcLSB); err != nil {
		s.Fatal("Failed to overwrite "+statefulLSBPath, err)
	}

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Restoring OS version")
		if statefulLSB != nil {
			if err := writeStatefulLSB(statefulLSB); err != nil {
				s.Fatal("Failed to overwrite "+statefulLSBPath, err)
			}
		} else {
			if err := os.Remove(statefulLSBPath); err != nil {
				testing.ContextLog(ctx, "Failed to delete", statefulLSBPath)
			}
		}
	}(ctx)

	// Update policies without target version.
	testing.ContextLog(ctx, "Serving policy with no target version")
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	testing.ContextLog(ctx, "Restarting update-engine")
	if err := upstart.RestartJob(ctx, "update-engine"); err != nil {
		s.Fatal("Failed to restart update-engine")
	}

	// Check if UpdateEngine service is up in DBus.
	dbus, err := dbusutil.SystemBus()
	if err != nil {
		s.Fatal("Failed to connect to dbus")
	}

	if err := dbusutil.WaitForService(ctx, dbus, "org.chromium.UpdateEngine"); err != nil {
		s.Fatal("UpdateEngine service is not running")
	}

	testing.ContextLog(ctx, "Checking for updates. New update expected")
	out, err := testexec.CommandContext(ctx,
		"update_engine_client",
		"--check_for_update").Output(testexec.DumpLogOnError)
	if err != nil || strings.Contains(string(out), updateCheckStarted) {
		s.Fatal("Failed to start checking for updates: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx,
			"update_engine_client",
			"--status").Output(testexec.DumpLogOnError)
		if err != nil || strings.Contains(string(out), updateCheckStarted) {
			return testing.PollBreak(errors.Wrap(err, "failed to check update-engine status"))
		}

		if strings.Contains(string(out), updateStatusChecking) {
			return errors.New("Still checking for updates")
		}

		if strings.Contains(string(out), updateStatusAvailable) {
			return nil
		}

		return testing.PollBreak(errors.New("Unknown update status: " + string(out)))
	}, nil); err != nil {
		s.Fatal("Failed to check for updates: ", err)
	}

	// Update policies with target version.
	testing.ContextLog(ctx, "Serving policy with target version")
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{
		&policy.DeviceTargetVersionPrefix{Val: targetVersionInPolicy},
	}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	testing.ContextLog(ctx, "Restarting update-engine")
	if err := upstart.RestartJob(ctx, "update-engine"); err != nil {
		s.Fatal("Failed to restart update-engine")
	}

	if err := dbusutil.WaitForService(ctx, dbus, "org.chromium.UpdateEngine"); err != nil {
		s.Fatal("UpdateEngine service is not running")
	}

	testing.ContextLog(ctx, "Checking for updates. No update expected")
	out, err = testexec.CommandContext(ctx,
		"update_engine_client",
		"--check_for_update").Output(testexec.DumpLogOnError)
	if err != nil || strings.Contains(string(out), updateCheckStarted) {
		s.Fatal("Failed to start checking for updates: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx,
			"update_engine_client",
			"--status").Output(testexec.DumpLogOnError)
		if err != nil || strings.Contains(string(out), updateCheckStarted) {
			return testing.PollBreak(errors.Wrap(err, "failed to check update-engine status"))
		}

		if strings.Contains(string(out), updateStatusChecking) {
			return errors.New("Still checking for updates")
		}

		if strings.Contains(string(out), updateStatusIdle) && strings.Contains(string(out), updateErrorNoUpdate) {
			return nil
		}

		return testing.PollBreak(errors.New("Unknown update status: " + string(out)))
	}, nil); err != nil {
		s.Fatal("Failed to check for updates: ", err)
	}
}

// writeStatefulLSB overwrites lsb-release in stateful partition.
func writeStatefulLSB(content map[string]string) error {
	output := new(bytes.Buffer)
	for key, value := range content {
		if _, err := fmt.Fprintf(output, "%s=%s\n", key, value); err != nil {
			return errors.Wrap(err, "failed to format lsb-release content")
		}
	}

	err := ioutil.WriteFile(statefulLSBPath, output.Bytes(), 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write lsb-release to %s", statefulLSBPath)
	}
	return nil
}
