// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encoding

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// regExpSSIM is the regexp to find the SSIM output in the tiny_ssim log.
var regExpSSIM = regexp.MustCompile(`\nSSIM: (\d+\.\d+)`)

// regExpPSNR is the regexp to find the PSNR output in the tiny_ssim log.
var regExpPSNR = regexp.MustCompile(`\nGlbPSNR: (\d+\.\d+)`)

// extractValue parses logFile using r and returns a single float64 match.
func extractValue(logFile string, r *regexp.Regexp) (value float64, err error) {
	b, err := ioutil.ReadFile(logFile)
	if err != nil {
		return 0.0, errors.Wrapf(err, "failed to read file %s", logFile)
	}

	matches := r.FindAllStringSubmatch(string(b), -1)
	if len(matches) != 1 {
		return 0.0, errors.Errorf("found %d matches in %q; want 1", len(matches), b)
	}

	matchString := matches[0][1]
	if value, err = strconv.ParseFloat(matchString, 64); err != nil {
		return 0.0, errors.Wrapf(err, "failed to parse value %q", matchString)
	}
	return
}

// CompareFiles decodes encodedFile using decoder and compares it with yuvFile using tiny_ssim.
func CompareFiles(ctx context.Context, decoder, yuvFile, encodedFile, outDir string, size coords.Size) (PSNR, SSIM float64, err error) {
	yuvFile2 := yuvFile + ".2"
	tf, err := os.Create(yuvFile2)
	if err != nil {
		return PSNR, SSIM, errors.Wrap(err, "failed to create a temporary YUV file")
	}
	defer os.Remove(yuvFile2)
	defer tf.Close()

	decodeCommand := []string{encodedFile}
	if decoder == "vpxdec" {
		decodeCommand = append(decodeCommand, "-o")
	}
	decodeCommand = append(decodeCommand, yuvFile2)
	testing.ContextLogf(ctx, "Executing %s %s", decoder, shutil.EscapeSlice(decodeCommand))

	decodeLogFileName := filepath.Join(outDir, decoder+".txt")
	decodeLogFile, err := os.Create(decodeLogFileName)
	if err != nil {
		return PSNR, SSIM, errors.Wrap(err, "failed to create log file")
	}
	defer decodeLogFile.Close()
	vpxCmd := testexec.CommandContext(ctx, decoder, decodeCommand...)
	vpxCmd.Stdout = decodeLogFile
	vpxCmd.Stderr = decodeLogFile
	if err := vpxCmd.Run(); err != nil {
		vpxCmd.DumpLog(ctx)
		return PSNR, SSIM, errors.Wrap(err, "decode failed")
	}

	ssimLogFileName := filepath.Join(outDir, "tiny_ssim.txt")
	ssimLogFile, err := os.Create(ssimLogFileName)
	if err != nil {
		return PSNR, SSIM, errors.Wrap(err, "failed to create log file")
	}
	defer ssimLogFile.Close()

	ssimCmd := testexec.CommandContext(ctx, "tiny_ssim", yuvFile, yuvFile2, strconv.Itoa(size.Width)+"x"+strconv.Itoa(size.Height))
	ssimCmd.Stdout = ssimLogFile
	ssimCmd.Stderr = ssimLogFile
	if err := ssimCmd.Run(testexec.DumpLogOnError); err != nil {
		return PSNR, SSIM, errors.Wrap(err, "failed to run tiny_ssim")
	}

	PSNR, err = extractValue(ssimLogFileName, regExpPSNR)
	if err != nil {
		return PSNR, SSIM, errors.Wrap(err, "failed to extract PSNR")
	}

	SSIM, err = extractValue(ssimLogFileName, regExpSSIM)
	if err != nil {
		return PSNR, SSIM, errors.Wrap(err, "failed to extract SSIM")
	}

	return PSNR, SSIM, nil
}
