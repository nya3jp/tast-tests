// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/taskswitchcuj"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TaskSwitchCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of the critical user journey of switching between applications in a high load environment",
		Contacts:     []string{"ramsaroop@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		Timeout:      20 * time.Minute,
		Vars:         []string{"mute"},
		Params: []testing.Param{
			{
				Name:              "clamshell",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				Fixture:           "loggedInToCUJUser",
				ExtraSoftwareDeps: []string{"android_p"},
				Val: taskswitchcuj.TaskSwitchTest{
					BrowserType: browser.TypeAsh,
				},
			},
			{
				Name:              "clamshell_vm",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "loggedInToCUJUser",
				Val: taskswitchcuj.TaskSwitchTest{
					BrowserType: browser.TypeAsh,
				},
			}, {
				Name:              "lacros_clamshell",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_p", "lacros"},
				Fixture:           "loggedInToCUJUserLacros",
				Val: taskswitchcuj.TaskSwitchTest{
					BrowserType: browser.TypeLacros,
				},
			}, {
				Name:              "lacros_clamshell_vm",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_vm", "lacros"},
				Fixture:           "loggedInToCUJUserLacros",
				Val: taskswitchcuj.TaskSwitchTest{
					BrowserType: browser.TypeLacros,
				},
			}, {
				Name:              "tablet",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "loggedInToCUJUser",
				Val: taskswitchcuj.TaskSwitchTest{
					Tablet:      true,
					BrowserType: browser.TypeAsh,
				},
			},
			{
				Name:              "tablet_vm",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "loggedInToCUJUser",
				Val: taskswitchcuj.TaskSwitchTest{
					Tablet:      true,
					BrowserType: browser.TypeAsh,
				},
			}, {
				Name:              "lacros_tablet",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_p", "lacros"},
				Fixture:           "loggedInToCUJUserLacros",
				Val: taskswitchcuj.TaskSwitchTest{
					Tablet:      true,
					BrowserType: browser.TypeLacros,
				},
			}, {
				Name:              "lacros_tablet_vm",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_vm", "lacros"},
				Fixture:           "loggedInToCUJUserLacros",
				Val: taskswitchcuj.TaskSwitchTest{
					Tablet:      true,
					BrowserType: browser.TypeLacros,
				},
			}, {
				Name:              "trace",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "loggedInToCUJUser",
				Val: taskswitchcuj.TaskSwitchTest{
					Tracing:     true,
					BrowserType: browser.TypeAsh,
				},
			}, {
				Name:              "validation",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "loggedInToCUJUser",
				Val: taskswitchcuj.TaskSwitchTest{
					Validation:  true,
					BrowserType: browser.TypeAsh,
				},
			}, {
				// Pilot test on "noibat" that has HDMI dongle installed.
				Name:              "noibat",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("noibat")),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "loggedInToCUJUser",
				Val: taskswitchcuj.TaskSwitchTest{
					BrowserType: browser.TypeAsh,
				},
			},
		},
	})
}

func TaskSwitchCUJ(ctx context.Context, s *testing.State) {
	taskswitchcuj.Run(ctx, s)
}
