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
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
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
		SoftwareDeps: []string{"chrome", "auto_update_stable"},
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
	// Reserve 3 seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Make a backup of lsb-release in stateful partition.
	statefulLSB, _ := lsbrelease.LoadFrom(statefulLSBPath)

	// Load lsb-release template from /etc/lsb-release.
	etcLSB, err := lsbrelease.LoadFrom(etcLSBPath)
	if err != nil {
		s.Fatal("Failed to load original "+etcLSBPath, err)
	}

	// Generate signed board name to receive updates.
	board, ok := etcLSB[lsbrelease.Board]
	if !ok {
		s.Fatal("Failed to get board from "+etcLSBPath, err)
	}
	signedBoard := board + "-signed-mp-v2keys"
	etcLSB[lsbrelease.Board] = signedBoard

	// Modify OS version.
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
	}(cleanupCtx)

	// Connect to DBus to check if UpdateEngine service is up.
	dbusConn, err := dbusutil.SystemBus()
	if err != nil {
		s.Fatal("Failed to connect to dbus")
	}

	// Update policies without target version.
	testing.ContextLog(ctx, "Testing without target version policy")
	if err := servePolicyAndCheckUpdates(ctx, fdms, cr, dbusConn, []policy.Policy{}, updateStatusAvailable); err != nil {
		s.Fatal("Failed to serve policy and check for updates: ", err)
	}

	// Update policies with target version.
	testing.ContextLog(ctx, "Testing with target version policy")
	if err := servePolicyAndCheckUpdates(ctx, fdms, cr, dbusConn, []policy.Policy{
		&policy.DeviceTargetVersionPrefix{Val: targetVersionInPolicy},
	}, updateStatusIdle, updateErrorNoUpdate); err != nil {
		s.Fatal("Failed to serve policy and check for updates: ", err)
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

// servePolicyAndCheckUpdates serves a new policy with or without target version
// and checks if update-engine finds any available updates.
func servePolicyAndCheckUpdates(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, dbusConn *dbus.Conn, policies []policy.Policy, expectedStatuses ...string) error {
	testing.ContextLog(ctx, "Serving new policy")
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, policies); err != nil {
		return errors.Wrap(err, "failed to update policies")
	}

	testing.ContextLog(ctx, "Restarting update-engine")
	if err := upstart.RestartJob(ctx, "update-engine"); err != nil {
		return errors.Wrap(err, "failed to restart update-engine")
	}

	if err := dbusutil.WaitForService(ctx, dbusConn, "org.chromium.UpdateEngine"); err != nil {
		return errors.Wrap(err, "update-engine service is not running")
	}

	testing.ContextLog(ctx, "Checking for updates")
	out, err := testexec.CommandContext(ctx,
		"update_engine_client",
		"--check_for_update").Output(testexec.DumpLogOnError)
	if err != nil || strings.Contains(string(out), updateCheckStarted) {
		return errors.Wrap(err, "failed to start checking for updates")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx,
			"update_engine_client",
			"--status").Output(testexec.DumpLogOnError)
		if err != nil || strings.Contains(string(out), updateCheckStarted) {
			return testing.PollBreak(errors.Wrap(err, "failed to check update-engine status"))
		}

		if strings.Contains(string(out), updateStatusChecking) {
			return errors.New("Update checking not finished yet")
		}

		containsExpectedStatuses := true
		for _, status := range expectedStatuses {
			if !strings.Contains(string(out), status) {
				containsExpectedStatuses = false
			}
		}
		if containsExpectedStatuses {
			return nil
		}

		return testing.PollBreak(errors.New("Unknown update status: " + string(out)))
	}, nil); err != nil {
		return errors.Wrap(err, "failed to check for updates")
	}

	return nil
}
