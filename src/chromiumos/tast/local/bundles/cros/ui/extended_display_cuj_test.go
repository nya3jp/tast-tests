// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"testing"
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
)

func TestExtendedDisplayParams(t *testing.T) {
	netflixWeb := `extendedDisplayCUJParam{
		ApplicationName: "netflix",
		ApplicationType: cuj.Web,
	}`
	params := []generatorParam{
		generatorParam{
			Tier:       cuj.Plus,
			Scenario:   "video",
			NameSuffix: "netflix_web",
			Timeout:    10 * time.Minute,
			ValParams:  netflixWeb,
		},
	}
	p, err := makeCUJCaseParam(t, params)
	if err != nil {
		t.Fatal("Failed to make CUJ case param: ", err)
	}
	genparams.Ensure(t, "extended_display_cuj.go", p)
}
