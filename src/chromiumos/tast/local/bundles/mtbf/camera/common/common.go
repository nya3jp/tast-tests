// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// RemoveJPGFiles remove jpg files.
func RemoveJPGFiles(s *testing.State) {
	removeFilesByExtension(s, "IMG", ".jpg")
}

// RemoveMKVFiles remove mkv files.
func RemoveMKVFiles(s *testing.State) {
	removeFilesByExtension(s, "VID", ".mkv")
}

// removeFilesByExtension remove files by extension.
func removeFilesByExtension(s *testing.State, prefix string, extension string) {
	downloadPath := "/home/chronos/user/Downloads/"

	d, err := os.Open(downloadPath)
	if err != nil {
		s.Error("Failed to remove file: ", err)
	}
	defer d.Close()

	files, err := d.Readdir(-1)
	if err != nil {
		s.Error("Failed to remove file: ", err)
	}

	for _, file := range files {
		if file.Mode().IsRegular() {
			fileName := file.Name()
			if strings.HasPrefix(fileName, prefix) && filepath.Ext(fileName) == extension {
				s.Log("Remove file : ", file.Name())
				if err := os.Remove(downloadPath + file.Name()); err != nil {
					s.Error("Failed to remove file: ", err)
				}
			}
		}
	}
}

// Relogin chrome.
func Relogin(ctx context.Context, s *testing.State) {
	s.Log("Start to relogin")
	testing.Sleep(ctx, 5*time.Second) // Sleep to make sure last login stable
	_, err := chrome.NewForLoginReuse(ctx, s)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeInit, err))
	}

	testing.Sleep(ctx, 5*time.Second)
}
