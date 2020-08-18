// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfutil

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

// Unzip an archive |srcFile| and copies the contained files at |destPath| in
// the original relative hierarchy
func Unzip(srcFile, destPath string) error {
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
