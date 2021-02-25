// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// This file contains helper functions related to files in a user's home directory.

// GetUserTestFilePath returns the full path of the given file under the given user's home dir.
func GetUserTestFilePath(ctx context.Context, util *CryptohomeClient, user, fileName string) (string, error) {
	userPath, err := util.GetHomeUserPath(ctx, user)
	if err != nil {
		return "", err
	}

	return filepath.Join(userPath, fileName), nil
}

// WriteUserTestContent writes the given content to the given file into the given user's home dir.
// The file is created if it doesn't exist.
func WriteUserTestContent(ctx context.Context, util *CryptohomeClient, cmdRunner CmdRunner, user, fileName, content string) error {
	testFile, err := GetUserTestFilePath(ctx, util, user, fileName)
	if err != nil {
		return err
	}

	if _, err := cmdRunner.Run(ctx, "sh", "-c", fmt.Sprintf("echo -n %q > %q", content, testFile)); err != nil {
		return err
	}

	return nil
}

// DoesUserTestFileExist checks and returns if the given test file exists in the given user's home dir.
func DoesUserTestFileExist(ctx context.Context, util *CryptohomeClient, cmdRunner CmdRunner, user, fileName string) (bool, error) {
	testFile, err := GetUserTestFilePath(ctx, util, user, fileName)
	if err != nil {
		return false, err
	}

	outBinary, err := cmdRunner.Run(ctx, "sh", "-c", fmt.Sprintf("[ -f %q ] && echo File; true", testFile))
	if err != nil {
		return false, err
	}

	out := strings.TrimSpace(string(outBinary))
	return out == "File", nil
}

// ReadUserTestContent reads content from the given file under the given user's home dir.
// Returns the file contents if the read succeeded or an error if there's anything wrong.
func ReadUserTestContent(ctx context.Context, util *CryptohomeClient, cmdRunner CmdRunner, user, fileName string) ([]byte, error) {
	testFile, err := GetUserTestFilePath(ctx, util, user, fileName)
	if err != nil {
		return nil, err
	}

	outBinary, err := cmdRunner.Run(ctx, "cat", testFile)
	if err != nil {
		return nil, err
	}

	return outBinary, nil
}
