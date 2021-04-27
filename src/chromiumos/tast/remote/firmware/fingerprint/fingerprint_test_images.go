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
	// GenTestImagesScript generates test images
	GenTestImagesScript = "gen_test_images.sh"
	// Futility is the futility executable name
	Futility = "futility"
	// BloonchipperDevKey is the path to the dev key
	BloonchipperDevKey = "fingerprint_dev_keys/bloonchipper/dev_key.pem"
	// DartmonkeyDevKey is the path to the dev key
	DartmonkeyDevKey = "fingerprint_dev_keys/dartmonkey/dev_key.pem"
	// NamiFPDevKey is the path to the dev key
	NamiFPDevKey = "fingerprint_dev_keys/nami_fp/dev_key.pem"
	// NocturneFPDevKey is the path to the dev key
	NocturneFPDevKey = "fingerprint_dev_keys/nocturne_fp/dev_key.pem"
)

const generatedImagesSubDirectory = "images"

// TestImageType specifies the test image variant
type TestImageType int

// TestImageData represents a firmware test image
type TestImageData struct {
	// Path is the absolute path to the firmware file on the DUT
	Path string
	// ROVersion is the RO version string
	ROVersion string
	// RWVersion is the RW version string
	RWVersion string
}

const (
	// TestImageTypeOriginal is the original firmware on the DUT
	TestImageTypeOriginal TestImageType = iota
	// TestImageTypeDev is a dev-key signed version of the firmware
	TestImageTypeDev
	// TestImageTypeCorruptFirstByte is a variant of the original firmware with the first byte changed
	TestImageTypeCorruptFirstByte
	// TestImageTypeCorruptLastByte is a variant of the original firmware with the last byte changed
	TestImageTypeCorruptLastByte
	// TestImageTypeDevRollbackZero is a dev-key signed version of the firmware with rollback set to zero
	TestImageTypeDevRollbackZero
	// TestImageTypeDevRollbackOne is a dev-key signed version of the firmware with rollback set to one
	TestImageTypeDevRollbackOne
	// TestImageTypeDevRollbackNine is a dev-key signed version of the firmware with rollback set to nine
	TestImageTypeDevRollbackNine
)

// TestImages maps a given test image type to data describing the image
type TestImages map[TestImageType]*TestImageData

// GenerateTestFirmwareImages generates a set of test firmware images from the firmware that is on the DUT.
func GenerateTestFirmwareImages(ctx context.Context, d *dut.DUT, fs *dutfs.Client, generateScript string, fpBoard FPBoardName, buildFWFile, dutTempDir string) (ret TestImages, retErr error) {
	testing.ContextLog(ctx, "Creating temp dir")
	serverTmpDir, err := ioutil.TempDir("", "*")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}

	testing.ContextLog(ctx, "Copying firmware from DUT to host")
	if err := linuxssh.GetFile(ctx, d.Conn(), buildFWFile, path.Join(serverTmpDir, path.Base(buildFWFile))); err != nil {
		return nil, errors.Wrap(err, "failed to get file")
	}

	testing.ContextLog(ctx, "Getting current working directory")
	pushd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get working dir")
	}

	testing.ContextLog(ctx, "Changing to temp dir")
	if err := os.Chdir(serverTmpDir); err != nil {
		return nil, errors.Wrapf(err, "failed to change dir to %q", serverTmpDir)
	}
	defer func() {
		testing.ContextLog(ctx, "Restoring original working directory")
		if err := os.Chdir(pushd); err != nil {
			ret = nil
			retErr = errors.Wrap(err, "failed to change directories")
		}
	}()

	testing.ContextLog(ctx, "Running script on host to generate firmware images")
	cmd := []string{generateScript, string(fpBoard), path.Base(buildFWFile)}
	if output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput(); err != nil {
		return nil, errors.Wrapf(err, "failed to run command: %q, output: %q", cmd, string(output))
	}

	fpBoardStr := string(fpBoard)
	images := TestImages{
		TestImageTypeOriginal:         &TestImageData{Path: fpBoardStr + ".bin"},
		TestImageTypeDev:              &TestImageData{Path: fpBoardStr + ".dev"},
		TestImageTypeCorruptFirstByte: &TestImageData{Path: fpBoardStr + "_corrupt_first_byte.bin"},
		TestImageTypeCorruptLastByte:  &TestImageData{Path: fpBoardStr + "_corrupt_last_byte.bin"},
		TestImageTypeDevRollbackZero:  &TestImageData{Path: fpBoardStr + ".dev.rb0"},
		TestImageTypeDevRollbackOne:   &TestImageData{Path: fpBoardStr + ".dev.rb1"},
		TestImageTypeDevRollbackNine:  &TestImageData{Path: fpBoardStr + ".dev.rb9"},
	}

	filesToCopy := make(map[string]string)
	for imageType, imageData := range images {
		dutFileName := filepath.Join(dutTempDir, generatedImagesSubDirectory, imageData.Path)
		filesToCopy[filepath.Join(serverTmpDir, generatedImagesSubDirectory, imageData.Path)] = dutFileName
		images[imageType].Path = dutFileName
	}

	testing.ContextLog(ctx, "Copying generating firmware images to DUT")
	if _, err := linuxssh.PutFiles(ctx, d.Conn(), filesToCopy, linuxssh.PreserveSymlinks); err != nil {
		return nil, errors.Wrapf(err, "failed to copy files from %q to %q", serverTmpDir, dutTempDir)
	}

	for _, imageData := range images {
		// Make sure that images were actually copied to DUT.
		exists, err := fs.Exists(ctx, imageData.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to check existence of %q", imageData.Path)
		}
		if !exists {
			return nil, errors.Errorf("expected file to exist, but it does not: %q", imageData.Path)
		}

		// Collect the version strings from each of the generated images.
		version, err := GetBuildROFirmwareVersion(ctx, d, fs, imageData.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get RO version from firmware file: %q", imageData.Path)
		}
		imageData.ROVersion = version

		version, err = GetBuildRWFirmwareVersion(ctx, d, fs, imageData.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get RW version from firmware file: %q", imageData.Path)
		}
		imageData.RWVersion = version
	}

	return images, nil
}
