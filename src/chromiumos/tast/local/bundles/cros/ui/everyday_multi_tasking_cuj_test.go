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

func TestEverydayMultiTaskingCUJParams(t *testing.T) {
	multiTaskWithBlueTooth := "true"
	multiTaskNoBlueTooth := "false"
	btSuffix := "bluetooth"

	params := []generatorParam{
		generatorParam{
			Tier:      cuj.Basic,
			Scenario:  "ytmusic",
			Timeout:   10 * time.Minute,
			ValParams: multiTaskNoBlueTooth,
		},
		generatorParam{
			Tier:       cuj.Basic,
			Scenario:   "ytmusic",
			NameSuffix: btSuffix,
			Timeout:    10 * time.Minute,
			ValParams:  multiTaskWithBlueTooth,
		},
		generatorParam{
			Tier:       cuj.Basic,
			Scenario:   "spotify",
			NameSuffix: btSuffix,
			Timeout:    10 * time.Minute,
			ValParams:  multiTaskWithBlueTooth,
		},
		generatorParam{
			Tier:      cuj.Plus,
			Scenario:  "ytmusic",
			Timeout:   15 * time.Minute,
			ValParams: multiTaskNoBlueTooth,
		},
		generatorParam{
			Tier:       cuj.Plus,
			Scenario:   "ytmusic",
			NameSuffix: btSuffix,
			Timeout:    15 * time.Minute,
			ValParams:  multiTaskWithBlueTooth,
		},
		generatorParam{
			Tier:       cuj.Plus,
			Scenario:   "spotify",
			NameSuffix: btSuffix,
			Timeout:    15 * time.Minute,
			ValParams:  multiTaskWithBlueTooth,
		},
	}
	p, err := makeCUJCaseParam(t, params)
	if err != nil {
		t.Fatal("Failed to make CUJ case param: ", err)
	}
	genparams.Ensure(t, "everyday_multi_tasking_cuj.go", p)
}
