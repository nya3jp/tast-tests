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
		name       string
		expected   []string
		optSetters []ActivityStartOptionSetter
	}{
		{
			"NoOptions",
			[]string{},
			[]ActivityStartOptionSetter{},
		},
		{
			"EnableDebugging",
			[]string{"-D"},
			[]ActivityStartOptionSetter{
				WithEnableDebugging(),
			},
		},
		{
			"EnableNativeDebugging",
			[]string{"-N"},
			[]ActivityStartOptionSetter{
				WithEnableNativeDebugging(),
			},
		},
		{
			"ForceStop",
			[]string{"-S"},
			[]ActivityStartOptionSetter{
				WithForceStop(),
			},
		},
		{
			"WaitForLaunch",
			[]string{"-W"},
			[]ActivityStartOptionSetter{
				WithWaitForLaunch(),
			},
		},
		{
			"SetIntentAction",
			[]string{"-a", "intentAction"},
			[]ActivityStartOptionSetter{
				WithIntentAction("intentAction"),
			},
		},
		{
			"SetDataURI",
			[]string{"-d", "dataURI"},
			[]ActivityStartOptionSetter{
				WithDataURI("dataURI"),
			},
		},
		{
			"SetUser",
			[]string{"--user", "user"},
			[]ActivityStartOptionSetter{
				WithUser("user"),
			},
		},
		{
			"SetDisplayID",
			[]string{"--display", "0"},
			[]ActivityStartOptionSetter{
				WithDisplayID(0),
			},
		},
		{
			"SetWindowingModeUndefined",
			[]string{"--windowingMode", "0"},
			[]ActivityStartOptionSetter{
				WithWindowingMode(WindowingModeUndefined),
			},
		},
		{
			"SetActivityTypeUndefined",
			[]string{"--activityType", "0"},
			[]ActivityStartOptionSetter{
				WithActivityType(ActivityTypeUndefined),
			},
		},
		{
			"AddExtraInt",
			[]string{"--ei", "extra", "0"},
			[]ActivityStartOptionSetter{
				WithExtraInt("extra", 0),
			},
		},
		{
			"AddExtraString",
			[]string{"--es", "extra", "string"},
			[]ActivityStartOptionSetter{
				WithExtraString("extra", "string"),
			},
		},
		{
			"AddExtraStringArray",
			[]string{"--esa", "extra", "string1,string2"},
			[]ActivityStartOptionSetter{
				WithExtraStringArray("extra", []string{"string1", "string2"}),
			},
		},
		{
			"AddExtraBool",
			[]string{"--ez", "extra", "false"},
			[]ActivityStartOptionSetter{
				WithExtraBool("extra", false),
			},
		},
		{
			"Composite",
			[]string{"-D", "-W", "-S", "--user", "user", "--es", "extra", "string",
				"--ei", "extra", "0"},
			[]ActivityStartOptionSetter{
				WithEnableDebugging(),
				WithWaitForLaunch(),
				WithForceStop(),
				WithUser("user"),
				WithExtraString("extra", "string"),
				WithExtraInt("extra", 0),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := makeActivityStartOptions()
			for _, setter := range tc.optSetters {
				setter(opts)
			}
			args := opts.buildStartCmdArgs()
			if !stringSlicesAreEqual(args, tc.expected) {
				t.Errorf("%v != %v", args, tc.expected)
			}
		})
	}
}
