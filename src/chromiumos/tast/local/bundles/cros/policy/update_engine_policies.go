// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

type updateEngineTestParam struct {
	// policyValues are the policies that need to be set.
	policyValues []policy.Policy
	// policyParam os the xml attribute that needs to be set by update_engine.
	policyParam string
	// testValue is the value for the policyParam attribute.
	testValue string

	// Some values are too generic or are always set, allow skipping the check when the policies are unset.
	// checkParam indicates whether to check for the xml attribute.
	checkParam bool
	// checkVal indicates whether to check for the value.
	checkVal bool
}

const (
	deviceTargetVersionSelectorVal = "0,1626155736-"
	deviceTargetVersionPrefixVal   = "1000."
	deviceReleaseLtsTagVal         = "lts"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UpdateEnginePolicies,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check of policies are properly propagating to update_engine by checking the logs",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromeEnrolledLoggedIn,
		Timeout:      1 * time.Minute,
		Params: []testing.Param{{
			Name: "device_target_version_selector",
			Val: &updateEngineTestParam{
				policyValues: []policy.Policy{&policy.DeviceTargetVersionSelector{Val: deviceTargetVersionSelectorVal}},
				testValue:    deviceTargetVersionSelectorVal,
				policyParam:  "targetversionselector",
				checkParam:   true,
				checkVal:     true,
			},
		}, {
			Name: "device_target_version_prefix",
			Val: &updateEngineTestParam{
				policyValues: []policy.Policy{&policy.DeviceTargetVersionPrefix{Val: deviceTargetVersionPrefixVal}},
				testValue:    deviceTargetVersionPrefixVal,
				policyParam:  "targetversionprefix",
				checkParam:   true,
				checkVal:     true,
			},
		}, {
			Name: "device_release_lts_tag",
			Val: &updateEngineTestParam{
				policyValues: []policy.Policy{&policy.DeviceReleaseLtsTag{Val: deviceReleaseLtsTagVal}},
				testValue:    deviceReleaseLtsTagVal,
				policyParam:  "ltstag",
				checkParam:   true,
			},
		}, {
			Name: "device_rollback_to_target_version",
			Val: &updateEngineTestParam{
				policyValues: []policy.Policy{
					&policy.DeviceTargetVersionPrefix{Val: deviceTargetVersionPrefixVal},
					&policy.DeviceRollbackToTargetVersion{Val: 2},
				},
				testValue:   "true",
				policyParam: "rollback_allowed",
			},
		}},
	})
}

const updateEngineLog = "/var/log/update_engine.log"

// clearAndUpdate restarts update engine, clears the logs and requests an update.
func clearAndUpdate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	if err := upstart.StopJob(ctx, "update-engine"); err != nil {
		return errors.Wrap(err, "failed to stop update_engine")
	}

	realLog, err := os.Readlink(updateEngineLog)
	if err != nil {
		return errors.Wrap(err, "failed to find the real update_engine log")
	}

	if err := os.Remove(realLog); err != nil {
		return errors.Wrap(err, "failed to clear the real update_engine log")
	}

	if err := os.Remove(updateEngineLog); err != nil {
		return errors.Wrap(err, "failed to clear the update_engine log")
	}

	if err := upstart.StartJob(ctx, "update-engine"); err != nil {
		return errors.Wrap(err, "failed to start update_engine")
	}

	// update_engine is not ready right after start.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Make sure update_engine_client does not hang.
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := testexec.CommandContext(ctx, "update_engine_client", "--check_for_update").Run(testexec.DumpLogOnError); err != nil {
			return err
		}

		return nil
	}, nil); err != nil {
		return errors.Wrap(err, "failed to trigger update check")
	}

	return nil
}

func UpdateEnginePolicies(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	param := s.Param().(*updateEngineTestParam)

	const waitTime = 10 * time.Second

	// Restart update-engine after clearing policies.
	defer upstart.RestartJob(ctx, "update-engine")
	defer policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{})

	// Set the policy and check that the attribute is set.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policyValues); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	if err := clearAndUpdate(ctx); err != nil {
		s.Fatal("Failed to trigger update request: ", err)
	}

	attributeEntry := param.policyParam + "=\"" + param.testValue + "\""
	s.Log("Waiting for the log entry to show up")
	var dat []byte
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		if dat, err = ioutil.ReadFile("/var/log/update_engine.log"); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to read update_engine logs"))
		}

		if !strings.Contains(string(dat), attributeEntry) {
			return errors.Errorf("%q not in the update_engine logs", attributeEntry)
		}

		return nil
	}, &testing.PollOptions{
		Timeout: waitTime,
	}); err != nil {
		s.Error("Could not find expected values: ", err)
	}

	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "set_log.txt"), dat, 0644); err != nil {
		s.Error("Failed to dump update_engine logs: ", err)
	}

	// Clear policies to make sure attribute is not always sent.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{}); err != nil {
		s.Fatal("Failed to clear policies: ", err)
	}

	if err := clearAndUpdate(ctx); err != nil {
		s.Fatal("Failed to trigger update request: ", err)
	}

	s.Log("Waiting for update_engine to have a chance to log")
	if err := testing.Sleep(ctx, waitTime); err != nil {
		s.Fatal("Failed to wait for messages: ", err)
	}

	dat, err := ioutil.ReadFile("/var/log/update_engine.log")
	if err != nil {
		s.Fatal("Failed to read update_engine logs: ", err)
	}

	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "unset_log.txt"), dat, 0644); err != nil {
		s.Error("Failed to dump update_engine logs: ", err)
	}

	if param.checkParam && strings.Contains(string(dat), param.policyParam) {
		s.Errorf("Unexpectedly found %q in the update_engine logs", param.policyParam)
	}

	if param.checkVal && strings.Contains(string(dat), param.testValue) {
		s.Errorf("Unexpectedly found test value %q in the update_engine logs", param.testValue)
	}
}
