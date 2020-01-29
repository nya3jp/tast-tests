// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/local/lacros"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Sanity,
		Desc:         "Tests basic lacros startup",
		Contacts:     []string{"erikchen@chromium.org", "hidehiko@chromium.org", "edcourtney@chromium.org", "lacros-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"disabled"},
		Params: []testing.Param{
			{
				Name:      "artifact",
				Pre:       lacros.StartedByArtifact(),
				Timeout:   7 * time.Minute,
				ExtraData: []string{lacros.ImageArtifact},
			},
		},
	})
}

func Sanity(ctx context.Context, s *testing.State) {
	_, err := s.PreValue().(lacros.PreData).Devsess.CreateTarget(ctx, "about:blank")
	if err != nil {
		s.Fatal("Failed to open new tab")
	}
}
