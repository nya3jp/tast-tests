// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cats

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBFLogin,
		Desc:         "Login for MTBF testing",
		Contacts:     []string{"xuechenwang@cienet.com"},
		SoftwareDeps: []string{"chrome", "android"},
		Pre:          arc.LoginReuse(),
		Vars:         []string{"cats.myOwnVar"},
		Params: []testing.Param{{
			Val:       1,
			ExtraAttr: []string{"group:mainline"},
		}},
		Data: []string{
			"format_m4a.m4a",
			"format_mp3.mp3",
			"format_ogg.ogg",
			"format_wav.wav",
			"test_video.mp4"},
		Timeout: 50 * time.Minute,
	})
}

// MTBFLogin just a empty case for login
func MTBFLogin(ctx context.Context, s *testing.State) {
	s.Log("MTBFLogin success")
	for _, file := range []string{
		"format_m4a.m4a",
		"format_mp3.mp3",
		"format_ogg.ogg",
		"format_wav.wav"} {
		if err := copyFile2Downloads(s, file, "audios"); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.VideoCopy, err, file, filepath.Join(filesapp.DownloadPath, "audios")))
		}
	}
	for _, file := range []string{
		"test_video.mp4"} {
		if err := copyFile2Downloads(s, file, "videos"); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.VideoCopy, err, file, filepath.Join(filesapp.DownloadPath, "videos")))
		}
	}
}

func copyFile2Downloads(s *testing.State, fileName, dirName string) error {
	dirLocation := filepath.Join(filesapp.DownloadPath, dirName)
	s.Logf("Try to copy %s to %s", fileName, dirLocation)
	if _, err := os.Stat(dirLocation); os.IsNotExist(err) {
		if err := os.Mkdir(dirLocation, 0755); err != nil {
			return err
		}
	}
	//just insure
	if err := os.Chown(dirLocation, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		return err
	}
	if err := os.Chmod(dirLocation, 0755); err != nil {
		return err
	}
	fileLocation := filepath.Join(dirLocation, fileName)
	//copy file
	if err := fsutil.CopyFile(s.DataPath(fileName), fileLocation); err != nil {
		return err
	}
	//by default is root:root, modify to chronos:chronos
	if err := os.Chown(fileLocation, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		return err
	}
	//check file if exist
	if _, err := os.Stat(fileLocation); os.IsNotExist(err) {
		return err
	}
	return nil
}
