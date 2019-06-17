// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
file_wiper.go provides the struct that wipes out and resotre a file on the DUT
*/

import (
	"context"

	"chromiumos/tast/errors"
)

// FileWiper wipes or restore a file on the DUT by renaming it.
type FileWiper struct {
	r CmdRunner
}

// Wipe wipes a file by moving it to a new filename.
// To be specific, it appends the filename with ".bak".
// Note: be careful if you do have a file with the backup name.
func (w *FileWiper) Wipe(ctx context.Context, path string) error {
	_, err := w.r.Run(ctx, "mv", path, path+".bak")
	if err != nil {
		return errors.Wrap(err, "failed to wipe out data")
	}
	return nil
}

// Restore restores a file by moving the backup file back to its original filename.
func (w *FileWiper) Restore(ctx context.Context, path string) error {
	_, err := w.r.Run(ctx, "mv", path+".bak", path)
	if err != nil {
		return errors.Wrap(err, "failed to restore data")
	}
	return nil
}

// NewFileWiper creates a new FileWiper with |r| running commands internally.
func NewFileWiper(r CmdRunner) *FileWiper {
	var w FileWiper
	w.r = r
	return &w
}
