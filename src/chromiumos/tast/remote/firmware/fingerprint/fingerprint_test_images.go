// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

const (
	// GenTestImagesScript generates test images.
	GenTestImagesScript = "gen_test_images.sh"
	// Futility is the futility executable name.
	Futility = "futility"
	// BloonchipperDevKey is the path to the dev key.
	BloonchipperDevKey = "fingerprint_dev_keys/bloonchipper/dev_key.pem"
	// DartmonkeyDevKey is the path to the dev key.
	DartmonkeyDevKey = "fingerprint_dev_keys/dartmonkey/dev_key.pem"
	// NamiFPDevKey is the path to the dev key.
	NamiFPDevKey = "fingerprint_dev_keys/nami_fp/dev_key.pem"
	// NocturneFPDevKey is the path to the dev key.
	NocturneFPDevKey = "fingerprint_dev_keys/nocturne_fp/dev_key.pem"
)

const generatedImagesSubDirectory = "images"

// TestImageType specifies the test image variant.
type TestImageType int

const (
	// TestImageTypeOriginal is the original firmware on the DUT.
	TestImageTypeOriginal TestImageType = iota
	// TestImageTypeDev is a dev-key signed version of the firmware.
	TestImageTypeDev
	// TestImageTypeCorruptFirstByte is a variant of the original firmware with the first byte changed.
	TestImageTypeCorruptFirstByte
	// TestImageTypeCorruptLastByte is a variant of the original firmware with the last byte changed.
	TestImageTypeCorruptLastByte
	// TestImageTypeDevRollbackZero is a dev-key signed version of the firmware with rollback set to zero.
	TestImageTypeDevRollbackZero
	// TestImageTypeDevRollbackOne is a dev-key signed version of the firmware with rollback set to one.
	TestImageTypeDevRollbackOne
	// TestImageTypeDevRollbackNine is a dev-key signed version of the firmware with rollback set to nine.
	TestImageTypeDevRollbackNine
)

// TestImages maps a given test image type to a path.
type TestImages map[TestImageType]string

// GenerateTestFirmwareImages generates a set of test firmware images from the firmware that is on the DUT.
func GenerateTestFirmwareImages(ctx context.Context, d *dut.DUT, fs *dutfs.Client, generateScript string, fpBoard FPBoardName, buildFWFile, dutTempDir string) (ret TestImages, retErr error) {
	testing.ContextLog(ctx, "Creating temp dir")
	serverTmpDir, err := ioutil.TempDir("", "*")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(serverTmpDir)

	testing.ContextLog(ctx, "Copying firmware from DUT to host")
	if err := linuxssh.GetFile(ctx, d.Conn(), buildFWFile, filepath.Join(serverTmpDir, filepath.Base(buildFWFile))); err != nil {
		return nil, errors.Wrap(err, "failed to get file")
	}

	testing.ContextLog(ctx, "Running script on host to generate firmware images")
	cmdStr := []string{generateScript, string(fpBoard), path.Base(buildFWFile)}
	cmd := exec.Command(cmdStr[0], cmdStr[1:]...)
	cmd.Dir = serverTmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, errors.Wrapf(err, "failed to run command: %q, output: %q", cmdStr, string(output))
	}

	fpBoardStr := string(fpBoard)
	images := TestImages{
		TestImageTypeOriginal:         fpBoardStr + ".bin",
		TestImageTypeDev:              fpBoardStr + ".dev",
		TestImageTypeCorruptFirstByte: fpBoardStr + "_corrupt_first_byte.bin",
		TestImageTypeCorruptLastByte:  fpBoardStr + "_corrupt_last_byte.bin",
		TestImageTypeDevRollbackZero:  fpBoardStr + ".dev.rb0",
		TestImageTypeDevRollbackOne:   fpBoardStr + ".dev.rb1",
		TestImageTypeDevRollbackNine:  fpBoardStr + ".dev.rb9",
	}

	filesToCopy := make(map[string]string)
	for imageType, fileName := range images {
		dutFileName := filepath.Join(dutTempDir, generatedImagesSubDirectory, fileName)
		filesToCopy[filepath.Join(serverTmpDir, generatedImagesSubDirectory, fileName)] = dutFileName
		images[imageType] = dutFileName
	}

	testing.ContextLog(ctx, "Copying generating firmware images to DUT")
	if _, err := linuxssh.PutFiles(ctx, d.Conn(), filesToCopy, linuxssh.PreserveSymlinks); err != nil {
		return nil, errors.Wrapf(err, "failed to copy files from %q to %q", serverTmpDir, dutTempDir)
	}

	// Make sure that images were actually copied to DUT.
	for _, fileName := range images {
		exists, err := fs.Exists(ctx, fileName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to check existence of %q", fileName)
		}
		if !exists {
			return nil, errors.Errorf("expected file to exist, but it does not: %q", fileName)
		}
	}

	return images, nil
}
