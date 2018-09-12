// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"encoding/base64"
	"io/ioutil"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// CopyFileToContainer copies a local file to the container's filesystem.
func CopyFileToContainer(ctx context.Context, cont *vm.Container, localPath string,
	containerPath string) error {
	testing.ContextLogf(ctx, "Copying local file %v to container %v", localPath, containerPath)
	// We base64 encode this and write it through terminal commands. We need to
	// base64 encode it since we are using the vsh command underneath which is a
	// terminal and binary control characters may interfere with its operation.
	debData, err := ioutil.ReadFile(localPath)
	if err != nil {
		return err
	}
	base64Deb := base64.StdEncoding.EncodeToString(debData)
	// TODO(jkardatzke): Switch this to using stdin to pipe the data once
	// https://crbug.com/885255 is fixed.
	cmd := cont.Command(ctx, "sh", "-c",
		"echo '"+base64Deb+"' | base64 --decode >"+containerPath)
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return err
	}
	return nil
}
