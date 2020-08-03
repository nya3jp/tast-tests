// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dlcservice provides a common interface for interacting with DLC.
//
// TODO(crbug/1112231):  Improve this infrastructure. E.g.:
//  - Checking installation status
//  - Non-blocking installation
//  - Querying what path the dlc is mounted at
package dlcservice

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// Install performs a blocking installation of the dlc with the provided id.
func Install(ctx context.Context, id string) error {
	if err := testexec.CommandContext(ctx, "dlcservice_util", "--id="+id, "--install").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to install %q DLC", id)
	}
	return nil
}
