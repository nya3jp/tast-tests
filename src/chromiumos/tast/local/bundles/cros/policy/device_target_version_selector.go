// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceTargetVersionSelector,
		Desc: "Check of DeviceTargetVersionSelector policy by checking update_engine logs",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromeEnrolledLoggedIn,
	})
}

func DeviceTargetVersionSelector(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	const testVal = "0,1626155736-"
	const updateEngineLog = "/var/log/update_engine.log"

	// Restart upstart after clearing policies.
	defer upstart.RestartJob(ctx, "update-engine")

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.DeviceTargetVersionSelector
	}{
		{
			name:  "tag",
			value: &policy.DeviceTargetVersionSelector{Val: testVal},
		},
		{
			name:  "unset",
			value: &policy.DeviceTargetVersionSelector{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Restart update engine and clear the logs.
			if err := upstart.StopJob(ctx, "update-engine"); err != nil {
				s.Fatal("Failed to stop update_engine: ", err)
			}

			realLog, err := os.Readlink(updateEngineLog)
			if err != nil {
				s.Fatal("Failed to find the real update_engine log: ", err)
			}

			if err := os.Remove(realLog); err != nil {
				s.Fatal("Failed to clear update_engine logs: ", err)
			}

			if err := os.Remove(updateEngineLog); err != nil {
				s.Fatal("Failed to clear update_engine logs: ", err)
			}

			if err := upstart.StartJob(ctx, "update-engine"); err != nil {
				s.Fatal("Failed to start update_engine: ", err)
			}

			// update_engine is not ready right after start.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				if err := testexec.CommandContext(ctx, "update_engine_client", "--check_for_update").Run(testexec.DumpLogOnError); err != nil {
					return err
				}

				return nil
			}, nil); err != nil {
				s.Fatal("Failed to trigger update check: ", err)
			}

			waitTime := 10 * time.Second
			if param.value.Val != "" {
				if err := testing.Poll(ctx, func(ctx context.Context) error {
					dat, err := ioutil.ReadFile("/var/log/update_engine.log")
					if err != nil {
						return testing.PollBreak(errors.Wrap(err, "failed to read update_engine logs"))
					}

					if !strings.Contains(string(dat), testVal) {
						return errors.Errorf("%q not in the update_engine logs", testVal)
					}

					return nil
				}, &testing.PollOptions{
					Timeout: waitTime,
				}); err != nil {
					s.Error("Could not find expected values: ", err)
				}
			} else {
				// Give update_engine time to log things.
				if err := testing.Sleep(ctx, waitTime); err != nil {
					s.Fatal("Failed to wait for messages: ", err)
				}

				dat, err := ioutil.ReadFile("/var/log/update_engine.log")
				if err != nil {
					s.Fatal("Failed to read update_engine logs: ", err)
				}

				if strings.Contains(string(dat), testVal) {
					s.Errorf("Unexpectedly found test value %q in the update_engine logs", testVal)
				}

				if strings.Contains(string(dat), "targetversionselector") {
					s.Error("Unexpectedly found targetversionselector in the update_engine logs")
				}
			}
		})
	}
}
