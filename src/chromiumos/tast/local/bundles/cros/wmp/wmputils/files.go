// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmputils contains utility functions for wmp tests.
package wmputils

import (
	"context"
	"os"
	"path/filepath"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
)

// HasScreenRecord checks if any screen record file is present in Download folder.
func HasScreenRecord(ctx context.Context) (bool, error) {
	re := regexp.MustCompile("Screen recording(.*?).webm")
	hasScreenRecord := false
	foundFileError := errors.New("stop walking because the target file is already found")
	if err := filepath.Walk(filesapp.DownloadPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrap(err, "failed to walk through files in Downloads folder")
		}
		if re.MatchString(info.Name()) {
			hasScreenRecord = true
			return foundFileError
		}
		return nil
	}); err != nil && err != foundFileError {
		return false, err
	}

	return hasScreenRecord, nil
}
