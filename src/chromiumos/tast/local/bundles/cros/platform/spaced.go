// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/spaced"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Spaced,
		Desc:     "Checks that spaced queries work",
		Contacts: []string{"sarthakkukreti@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func checkDiskSpaceQueries(ctx context.Context, spaced *spaced.Client) error {
	pathString := "/mnt/stateful_partition/"
	freeDiskSpace, err := spaced.GetFreeDiskSpace(ctx, pathString)
	if err != nil {
		return errors.Wrap(err, "failed to query free disk space")
	}
	testing.ContextLog(ctx, "GetFreeDiskSpace returns: ", freeDiskSpace)

	totalDiskSpace, err := spaced.GetTotalDiskSpace(ctx, pathString)
	if err != nil {
		return errors.Wrap(err, "failed to query total disk space")
	}
	testing.ContextLog(ctx, "GetTotalDiskSpace returns: ", totalDiskSpace)

	return nil
}

func Spaced(ctx context.Context, s *testing.State) {
	spaced, err := spaced.NewClient(ctx)
	if err != nil {
		s.Fatal("Failed to create spaced client: ", err)
	}

	// Check D-Bus queries.
	if err := checkDiskSpaceQueries(ctx, spaced); err != nil {
		s.Fatal("Querying memory status failed: ", err)
	}
}
