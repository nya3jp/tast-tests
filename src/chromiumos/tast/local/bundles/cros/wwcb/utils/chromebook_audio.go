// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils provides funcs to cleanup folders in ChromeOS.
package utils

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// AudioRecord execute cmd to record audio in Chromebook, then transfer record audio file to tast env
func AudioRecord(ctx context.Context, s *testing.State, duration int) error {

	// static path in chromebook
	rawPath := filepath.Join(filesapp.DownloadPath, "audio_record.raw")

	// can't add sh -c, have no idea why
	// record audio in .raw
	recordAudioCmd := testexec.CommandContext(ctx, "cras_test_client", "-C", rawPath, "--duration", fmt.Sprintf("%d", duration))
	rawFile, err := recordAudioCmd.Output(testexec.DumpLogOnError)
	if err != nil || rawFile == nil {
		return errors.Wrapf(err, "%q failed", shutil.EscapeSlice(recordAudioCmd.Args))
	}

	// convert to .wav
	wavPath := filepath.Join(filesapp.DownloadPath, "audio_record.wav")
	convertFileCmd := testexec.CommandContext(ctx, "sox", "-t", "raw", "-r", "48000", "-b", "16", "-c", "2", "-e", "Signed-integer", rawPath, wavPath)
	wavFile, err := convertFileCmd.Output(testexec.DumpLogOnError)
	if err != nil || wavFile == nil {
		return errors.Wrapf(err, "%q failed", shutil.EscapeSlice(convertFileCmd.Args))
	}

	// transfer file to tast env
	dir, ok := testing.ContextOutDir(ctx)

	if ok && dir != "" {
		if _, err := os.Stat(dir); err == nil {
			testing.ContextLogf(ctx, "Saving audio record to %s", dir)

			// read file
			b, err := ioutil.ReadFile(wavPath)
			if err != nil {
				return err
			}

			// write filePath to result folder
			filePath := filepath.Join(s.OutDir(), "audio_record.wav")
			if err := ioutil.WriteFile(filePath, b, 0644); err != nil {
				return errors.Wrapf(err, "failed to dump bytes to %s", filePath)
			}

		}
	}

	return nil
}
