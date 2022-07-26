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
		Desc:         "Measures the performance of the critical user journey for task switching",
		Contacts:     []string{"ramsaroop@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		Timeout:      25 * time.Minute,
		Vars:         []string{"mute"},
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				Fixture:           "loggedInToCUJUser",
				ExtraSoftwareDeps: []string{"android_p"},
				Val: taskswitchcuj.TaskSwitchTest{
					BrowserType: browser.TypeAsh,
				},
			},
			{
				Name:              "vm",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "loggedInToCUJUser",
				Val: taskswitchcuj.TaskSwitchTest{
					BrowserType: browser.TypeAsh,
				},
			}, {
				Name:              "lacros",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_p", "lacros"},
				Fixture:           "loggedInToCUJUserLacros",
				Val: taskswitchcuj.TaskSwitchTest{
					BrowserType: browser.TypeLacros,
				},
			}, {
				Name:              "lacros_vm",
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
					BrowserType: browser.TypeAsh,
					Tablet:      true,
				},
			},
			{
				Name:              "tablet_vm",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "loggedInToCUJUser",
				Val: taskswitchcuj.TaskSwitchTest{
					BrowserType: browser.TypeAsh,
					Tablet:      true,
				},
			}, {
				Name:              "lacros_tablet",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_p", "lacros"},
				Fixture:           "loggedInToCUJUserLacros",
				Val: taskswitchcuj.TaskSwitchTest{
					BrowserType: browser.TypeLacros,
					Tablet:      true,
				},
			}, {
				Name:              "lacros_tablet_vm",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_vm", "lacros"},
				Fixture:           "loggedInToCUJUserLacros",
				Val: taskswitchcuj.TaskSwitchTest{
					BrowserType: browser.TypeLacros,
					Tablet:      true,
				},
			}, {
				Name:              "tracing",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "loggedInToCUJUser",
				Val: taskswitchcuj.TaskSwitchTest{
					BrowserType: browser.TypeAsh,
					Tracing:     true,
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
