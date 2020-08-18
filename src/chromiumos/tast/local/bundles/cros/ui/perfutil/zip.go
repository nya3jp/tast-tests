// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfutil

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
)

// Unzip an archive |srcFile| and copy its content to |destPath| in its original
// relative hierarchy. If unzipping fails, the function returns the error and
// any temporarily created directories and files will be removed to restore the
// original state of the file system.
func Unzip(srcFile, destPath string) error {
	if unzipErr := unzip(srcFile, destPath); unzipErr != nil {
		if removeErr := os.RemoveAll(destPath); removeErr != nil {
			return errors.Errorf("%w, %v", unzipErr, removeErr.Error())
		}
		return unzipErr
	}
	return nil
}

// unzip an archive |srcFile| and copy its content to |destPath| in its original
// relative hierarchy. Unzipping stops if any error is encountered.
func unzip(srcFile, destPath string) error {
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}
	reader, err := zip.OpenReader(srcFile)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(destPath, file.Name)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, file.Mode()); err != nil {
				return err
			}
			continue
		}
		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer fileReader.Close()
		destFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		defer destFile.Close()
		if _, err := io.Copy(destFile, fileReader); err != nil {
			return err
		}
	}
	return nil
}
