// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package faillog provides a way to record logs on test failure.
package faillog

import (
	"path/filepath"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome/lacros/launcher"
)

// Save stores Lacros related log files into outdir.
func Save(hasError func() bool, l *launcher.LacrosChrome, outdir string) error {
	if !hasError() {
		return nil
	}

	return fsutil.CopyFile(l.LogFile(), filepath.Join(outdir, "lacros.log"))
}
