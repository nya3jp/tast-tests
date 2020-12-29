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

func TestQuickCheckCUJ2Params(t *testing.T) {
	params := []cuj.GeneratorParam{
		cuj.GeneratorParam{
			Tier:     cuj.Basic,
			Scenario: "unlock",
			Timeout:  10 * time.Minute,
		},
		cuj.GeneratorParam{
			Tier:     cuj.Basic,
			Scenario: "wakeup",
			Timeout:  10 * time.Minute,
		},
	}
	genparams.Ensure(t, "quick_check_cuj2.go", cuj.MakeCUJCaseParam(t, params))
}
