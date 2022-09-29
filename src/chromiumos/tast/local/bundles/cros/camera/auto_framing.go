// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/testing"
)

const autoFramingTestImageFile = "person_4032x3024.nv12"

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutoFraming,
		Desc:         "Auto-framing core pipeline smoke test",
		Contacts:     []string{"kamesan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"camera_feature_auto_framing"},
		Data:         []string{autoFramingTestImageFile, autoFramingTestImageFile + ".json"},
		Timeout:      4 * time.Minute,
	})
}

func AutoFraming(ctx context.Context, s *testing.State) {
	const gtestExecutable = "auto_framing_test"
	if _, err := gtest.New(
		gtestExecutable,
		gtest.Logfile(filepath.Join(s.OutDir(), gtestExecutable+".log")),
		gtest.ExtraArgs("--test_image_path="+s.DataPath(autoFramingTestImageFile)),
	).Run(ctx); err != nil {
		s.Errorf("Failed to run %v: %v", gtestExecutable, err)
	}
}
