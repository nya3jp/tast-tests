// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"testing"
)

func stringSlicesAreEqual(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true
}

func TestActivityStartOptions(t *testing.T) {

	for _, tc := range []struct {
		name      string
		expected  []string
		buildOpts func(*ActivityStartOptions)
	}{
		{
			"NoOptions",
			[]string{},
			func(opts *ActivityStartOptions) {},
		},
		{
			"EnableDebugging",
			[]string{"-D"},
			func(opts *ActivityStartOptions) {
				opts.EnableDebugging()
			},
		},
		{
			"EnableNativeDebugging",
			[]string{"-N"},
			func(opts *ActivityStartOptions) {
				opts.EnableNativeDebugging()
			},
		},
		{
			"ForceStop",
			[]string{"-S"},
			func(opts *ActivityStartOptions) {
				opts.ForceStop()
			},
		},
		{
			"WaitForLaunch",
			[]string{"-W"},
			func(opts *ActivityStartOptions) {
				opts.WaitForLaunch()
			},
		},
		{
			"SetIntentAction",
			[]string{"-a", "intentAction"},
			func(opts *ActivityStartOptions) {
				opts.SetIntentAction("intentAction")
			},
		},
		{
			"SetIntentAction",
			[]string{"-a", "intentAction"},
			func(opts *ActivityStartOptions) {
				opts.SetIntentAction("intentAction")
			},
		},
		{
			"SetDataURI",
			[]string{"-d", "dataURI"},
			func(opts *ActivityStartOptions) {
				opts.SetDataURI("dataURI")
			},
		},
		{
			"SetUser",
			[]string{"--user", "user"},
			func(opts *ActivityStartOptions) {
				opts.SetUser("user")
			},
		},
		{
			"SetWindowingModeUndefined",
			[]string{"--windowingMode", "0"},
			func(opts *ActivityStartOptions) {
				opts.SetWindowingMode(WindowingModeUndefined)
			},
		},
		{
			"SetActivityTypeUndefined",
			[]string{"--activityType", "0"},
			func(opts *ActivityStartOptions) {
				opts.SetActivityType(ActivityTypeUndefined)
			},
		},
		{
			"AddExtraInt",
			[]string{"--ei", "extra", "0"},
			func(opts *ActivityStartOptions) {
				opts.AddExtraInt("extra", 0)
			},
		},
		{
			"AddExtraString",
			[]string{"--es", "extra", "string"},
			func(opts *ActivityStartOptions) {
				opts.AddExtraString("extra", "string")
			},
		},
		{
			"AddExtraStringArray",
			[]string{"--esa", "extra", "string1,string2"},
			func(opts *ActivityStartOptions) {
				opts.AddExtraStringArray("extra", []string{"string1", "string2"})
			},
		},
		{
			"AddExtraBool",
			[]string{"--ez", "extra", "false"},
			func(opts *ActivityStartOptions) {
				opts.AddExtraBool("extra", false)
			},
		},
		{
			"Composite",
			[]string{"-D", "-W", "-S", "--user", "user", "--es", "extra", "string",
				"--ei", "extra", "0"},
			func(opts *ActivityStartOptions) {
				opts.EnableDebugging()
				opts.WaitForLaunch()
				opts.ForceStop()
				opts.SetUser("user")
				opts.AddExtraString("extra", "string")
				opts.AddExtraInt("extra", 0)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := MakeActivityStartOptions()
			tc.buildOpts(opts)
			args := opts.buildStartCmd()
			if !stringSlicesAreEqual(args, tc.expected) {
				t.Errorf("%v != %v", args, tc.expected)
			}
		})
	}
}
