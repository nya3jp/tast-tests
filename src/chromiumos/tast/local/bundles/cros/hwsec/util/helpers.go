// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
)

// This file contains some shared helper functions for local hwsec bundle tests.

// GetUserTestFilePath returns the full path of the given file under the given user's home dir.
func GetUserTestFilePath(ctx context.Context, user string, fileName string) (string, error) {
	userPath, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		return "", err
	}

	return filepath.Join(userPath, fileName), nil
}

// WriteUserTestContent writes the given content to the given file into the given user's home dir.
// The file is created if it doesn't exist.
func WriteUserTestContent(ctx context.Context, user string, fileName string, content []byte) error {
	testFile, err := GetUserTestFilePath(ctx, user, fileName)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(testFile, content, 0644)
}

// DoesUserTestFileExist checks and returns if the given test file exists in the given user's home dir.
func DoesUserTestFileExist(ctx context.Context, user string, fileName string) (bool, error) {
	testFile, err := GetUserTestFilePath(ctx, user, fileName)
	if err != nil {
		return false, err
	}

	fileInfo, err := os.Stat(testFile)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if fileInfo.IsDir() {
		return false, errors.Errorf("%s is a dir", testFile)
	}

	return true, nil
}
