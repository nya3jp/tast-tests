// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        RemoteFileSystem,
		Desc:        "Demonstrates how to access remote file system",
		Contacts:    []string{"tast-owners@google.com"},
		Attr:        []string{"group:mainline", "informational"},
		ServiceDeps: []string{dutfs.ServiceName},
	})
}

func RemoteFileSystem(ctx context.Context, s *testing.State) {
	fs, err := dutfs.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to dial to DUT for remote file system: ", err)
	}
	defer fs.Close(ctx)

	const dir = "/mnt/stateful_partition"
	fis, err := fs.ReadDir(ctx, dir)
	if err != nil {
		s.Fatalf("Failed to list files at %s: %v", dir, err)
	}

	s.Logf("Files under %s:", dir)
	for _, fi := range fis {
		s.Log("  ", fi.Name())
	}
}
