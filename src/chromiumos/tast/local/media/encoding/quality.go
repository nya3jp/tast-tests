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

// Decoder is a command line decoder that can be used in CompareFiles.
type Decoder string

const (
	// OpenH264Decoder is an H264 decoder built from https://github.com/cisco/openh264.
	OpenH264Decoder Decoder = "openh264dec"
	// LibvpxDecoder is a VP8 and VP9 decoder built from https://chromium.googlesource.com/webm/libvpx/.
	LibvpxDecoder Decoder = "vpxdec"
)

// regExpSSIM is the regexp to find the SSIM output in the tiny_ssim log.
var regExpSSIM = regexp.MustCompile(`\nSSIM: (\d+\.\d+)`)

// regExpPSNR is the regexp to find the PSNR output in the tiny_ssim log.
var regExpPSNR = regexp.MustCompile(`\nGlbPSNR: (\d+\.\d+)`)

// extractValues parses logFile using regExps and returns matched float64 values.
func extractValues(logFile string, regExps []*regexp.Regexp) (values []float64, err error) {
	b, err := ioutil.ReadFile(logFile)
	if err != nil {
		return values, errors.Wrapf(err, "failed to read file %s", logFile)
	}

	for _, r := range regExps {
		matches := r.FindAllStringSubmatch(string(b), -1)
		if len(matches) != 1 {
			return values, errors.Errorf("found %d matches for %s in %q; want 1", len(matches), r.String(), b)
		}

		matchString := matches[0][1]
		value, err := strconv.ParseFloat(matchString, 64)
		if err != nil {
			return values, errors.Wrapf(err, "failed to parse value %q", matchString)
		}
		values = append(values, value)
	}
	return
}

// CompareFiles decodes encodedFile using decoder and compares it with yuvFile using tiny_ssim.
// PSNR and SSIM are returned on success.
// Caveats: This creates text files in outDir and writes them. Calling twice overwrites the files.
func CompareFiles(ctx context.Context, decoder Decoder, yuvFile, encodedFile, outDir string, size coords.Size) (psnr, ssim float64, err error) {
	yuvFile2, err := CreatePublicTempFile(filepath.Base(yuvFile) + ".2")
	if err != nil {
		return psnr, ssim, errors.Wrap(err, "failed to create a temporary YUV file")
	}
	defer func() {
		yuvFile2.Close()
		os.Remove(yuvFile2.Name())
	}()

	decodeCommand := []string{encodedFile}
	if decoder == LibvpxDecoder {
		decodeCommand = append(decodeCommand, "-o")
	}
	decodeCommand = append(decodeCommand, yuvFile2.Name())
	testing.ContextLogf(ctx, "Executing %s %s", decoder, shutil.EscapeSlice(decodeCommand))

	decodeLogFileName := filepath.Join(outDir, string(decoder)+".txt")
	decodeLogFile, err := os.Create(decodeLogFileName)
	if err != nil {
		return psnr, ssim, errors.Wrap(err, "failed to create log file")
	}
	defer decodeLogFile.Close()
	vpxCmd := testexec.CommandContext(ctx, string(decoder), decodeCommand...)
	vpxCmd.Stdout = decodeLogFile
	vpxCmd.Stderr = decodeLogFile
	if err := vpxCmd.Run(testexec.DumpLogOnError); err != nil {
		return psnr, ssim, errors.Wrap(err, "decode failed")
	}

	ssimLogFileName := filepath.Join(outDir, "tiny_ssim.txt")
	ssimLogFile, err := os.Create(ssimLogFileName)
	if err != nil {
		return psnr, ssim, errors.Wrap(err, "failed to create log file")
	}
	defer ssimLogFile.Close()

	ssimCmd := testexec.CommandContext(ctx, "tiny_ssim", yuvFile, yuvFile2.Name(), strconv.Itoa(size.Width)+"x"+strconv.Itoa(size.Height))
	ssimCmd.Stdout = ssimLogFile
	if err := ssimCmd.Run(testexec.DumpLogOnError); err != nil {
		return psnr, ssim, errors.Wrap(err, "failed to run tiny_ssim")
	}

	values, err := extractValues(ssimLogFileName, []*regexp.Regexp{regExpPSNR, regExpSSIM})
	if err != nil {
		return psnr, ssim, errors.Wrap(err, "failed to extract PSNR and SSIM")
	}
	psnr = values[0]
	ssim = values[1]

	return psnr, ssim, nil
}
