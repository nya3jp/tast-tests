// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"path/filepath"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/testing"
)

func filePrepare(ctx context.Context, s *testing.State, fileNames []string, class string) {
	downloadPath := "/home/chronos/user/Downloads/"
	files := make(map[string]string)

	for _, file := range fileNames {
		src := s.DataPath(file)                          // local file path.
		dest := filepath.Join(downloadPath, class, file) // absolute path on DUT.
		files[src] = dest
	}

	if err := s.DUT().PushFiles(ctx, files); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.GRPCWriteFileErr, err))
	}
}

// AudioFilesPrepare prepares audio files and put it under /home/chronos/user/Downloads/audios folder
func AudioFilesPrepare(ctx context.Context, s *testing.State, fileNames []string) {
	filePrepare(ctx, s, fileNames, "audios")
}

// VideoFilesPrepare prepares video files and put it under /home/chronos/user/Downloads/videos folder
func VideoFilesPrepare(ctx context.Context, s *testing.State, fileNames []string) {
	filePrepare(ctx, s, fileNames, "videos")
}

// DataFilesPrepare prepares files and put it under /home/chronos/user/Downloads/data folder
func DataFilesPrepare(ctx context.Context, s *testing.State, fileNames []string) {
	filePrepare(ctx, s, fileNames, "data")
}

// Add1SecondForURL adds a start play time for one second of youtube url.
func Add1SecondForURL(url string) (youtube string) {
	youtube = url + "&t=1"
	return youtube
}
