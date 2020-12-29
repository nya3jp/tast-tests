// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

// Refer to cuj/test_params.go for the details of how to use this unit test.

import (
	"testing"
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
)

func TestTabSwitchCUJ2Params(t *testing.T) {
	params := []generatorParam{
		generatorParam{
			Tier:    cuj.Basic,
			Pre:     "wpr.RemoteReplayMode()",
			Timeout: 30 * time.Minute,
		},
		generatorParam{
			Tier:    cuj.Plus,
			Pre:     "wpr.RemoteReplayMode()",
			Timeout: 35 * time.Minute,
		},
		generatorParam{
			Tier:    cuj.Premium,
			Pre:     "wpr.RemoteReplayMode()",
			Timeout: 40 * time.Minute,
		},
		generatorParam{
			Tier:      cuj.Basic,
			Scenario:  "local",
			ExtraData: "[]string{tabswitchcuj.WPRArchiveName}",
			Pre:       "wpr.ReplayMode(tabswitchcuj.WPRArchiveName)",
			Timeout:   30 * time.Minute,
		},
		generatorParam{
			Tier:      cuj.Plus,
			Scenario:  "local",
			ExtraData: "[]string{tabswitchcuj.WPRArchiveName}",
			Pre:       "wpr.ReplayMode(tabswitchcuj.WPRArchiveName)",
			Timeout:   35 * time.Minute,
		},
		generatorParam{
			Tier:      cuj.Premium,
			ExtraData: "[]string{tabswitchcuj.WPRArchiveName}",
			Scenario:  "local",
			Pre:       "wpr.ReplayMode(tabswitchcuj.WPRArchiveName)",
			Timeout:   40 * time.Minute,
		},
	}
	p, err := makeCUJCaseParam(t, params)
	if err != nil {
		t.Fatal("Failed to make CUJ case param: ", err)
	}
	genparams.Ensure(t, "tab_switch_cuj2.go", p)
}
