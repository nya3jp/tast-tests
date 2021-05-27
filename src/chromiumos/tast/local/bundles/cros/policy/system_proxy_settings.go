// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemProxySettings,
		Desc: "Test setting the SystemProxySettings policy by checking if the System-proxy daemon and worker processes are running",
		Contacts: []string{
			"acostinas@google.com",
			"hugobenichi@chromium.org",
			"omorsi@chromium.org",
			"pmarko@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromeEnrolledLoggedIn",
	})
}

func SystemProxySettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	for _, param := range []struct {
		name  string                      // name is the subtest name
		value *policy.SystemProxySettings // value is the policy value
	}{
		{
			name: "enabled",
			value: &policy.SystemProxySettings{
				Val: &policy.SystemProxySettingsValue{
					SystemProxyEnabled:           true,
					SystemServicesPassword:       "********",
					SystemServicesUsername:       "********",
					PolicyCredentialsAuthSchemes: []string{},
				},
			},
		},
		{
			name: "disabled",
			value: &policy.SystemProxySettings{
				Val: &policy.SystemProxySettingsValue{
					SystemProxyEnabled:           false,
					SystemServicesPassword:       "********",
					SystemServicesUsername:       "********",
					PolicyCredentialsAuthSchemes: []string{},
				},
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Wait for 5 seconds to allow system-proxy to start the worker processe which authenticate OS level traffic.
			testing.Sleep(ctx, 5*time.Second)

			const (
				mainBinPath   = "system_proxy"
				workerBinPath = "system_proxy_worker"
			)
			if param.value.Val.SystemProxyEnabled {
				if isRunning, err := processRunning(mainBinPath); err != nil {
					s.Errorf("Failed to determine if %s is running: %v", mainBinPath, err)
				} else if !isRunning {
					s.Error("Main process is not running")
				}

				if isRunning, err := processRunning(workerBinPath); err != nil {
					s.Errorf("Failed to determine if %s is running: %v", workerBinPath, err)
				} else if !isRunning {
					s.Error("Worker process is not running")
				}
			} else {
				if isRunning, err := processRunning(mainBinPath); err != nil {
					s.Errorf("Failed to determine if %s is running: %v", mainBinPath, err)
				} else if isRunning {
					s.Error("System-proxy running although disabled by policy")
				}
			}
		})
	}
}

// processRunning checks if a process named procName is running.
func processRunning(procName string) (bool, error) {
	ps, err := process.Processes()
	if err != nil {
		return false, err
	}
	for _, p := range ps {
		if n, err := p.Name(); err == nil && n == procName {
			return true, nil
		}
	}
	return false, nil
}
