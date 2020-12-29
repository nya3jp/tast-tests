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

func TestVideoCUJ2Params(t *testing.T) {
	youtubeWeb := `videoCUJParam{
		ApplicationName: "youtube",
		ApplicationType: cuj.Web,
	}`
	youtubeWebTest := "youtube_web"
	youtubeApp := `videoCUJParam{
		ApplicationName: "youtube",
		ApplicationType: cuj.APP,
	}`
	youtubeAppTest := "youtube_app"
	netflixWeb := `videoCUJParam{
		ApplicationName: "netflix",
		ApplicationType: cuj.Web,
	}`
	netflixWebTest := "netflix_web"

	params := []generatorParam{
		generatorParam{
			Tier:      cuj.Basic,
			Timeout:   10 * time.Minute,
			ValParams: youtubeWeb,
			Scenario:  youtubeWebTest,
		},
		generatorParam{
			Tier:      cuj.Plus,
			Timeout:   10 * time.Minute,
			ValParams: youtubeWeb,
			Scenario:  youtubeWebTest,
		},
		generatorParam{
			Tier:      cuj.Basic,
			Timeout:   10 * time.Minute,
			ValParams: netflixWeb,
			Scenario:  netflixWebTest,
		},
		generatorParam{
			Tier:      cuj.Plus,
			Timeout:   10 * time.Minute,
			ValParams: netflixWeb,
			Scenario:  netflixWebTest,
		},
		generatorParam{
			Tier:      cuj.Basic,
			Timeout:   10 * time.Minute,
			ValParams: youtubeApp,
			Scenario:  youtubeAppTest,
		},
		generatorParam{
			Tier:      cuj.Plus,
			Timeout:   10 * time.Minute,
			ValParams: youtubeApp,
			Scenario:  youtubeAppTest,
		},
	}
	p, err := makeCUJCaseParam(t, params)
	if err != nil {
		t.Fatal("Failed to make CUJ case param: ", err)
	}
	genparams.Ensure(t, "video_cuj2.go", p)
}
