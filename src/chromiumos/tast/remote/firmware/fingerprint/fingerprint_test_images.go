// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"
	"encoding/binary"
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

const (
	generatedImagesSubDirectory = "images"
	versionStringLenBytes       = 32
	rollbackSizeBytes           = 4
)

// TestImageType specifies the test image variant.
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

// TestImages maps a given test image type to data describing the image.
type TestImages map[TestImageType]*TestImageData

func readFileAtOffset(fileName string, data []byte, offset int64) error {
	f, err := os.OpenFile(fileName, os.O_RDONLY, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to open file: %q", fileName)
	}
	defer f.Close()

	if _, err := f.ReadAt(data, offset); err != nil {
		return errors.Wrapf(err, "failed to read from offset: %v", offset)
	}

	return nil
}

func writeFileAtOffset(fileName string, data []byte, offset int64) error {
	f, err := os.OpenFile(fileName, os.O_RDWR, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to open file: %q", fileName)
	}
	defer f.Close()

	if _, err := f.WriteAt(data, offset); err != nil {
		return errors.Wrapf(err, "failed to write data: %v to offset: %v", data, offset)
	}

	return nil
}

// createVersionStringWithSuffix returns a new copy of version with the last bytes
// replaced by suffix.
func createVersionStringWithSuffix(suffix string, version []byte) ([]byte, error) {
	if len(version) != versionStringLenBytes {
		return nil, errors.Errorf("incorrect version size, actual: %d, expected: %d", len(version), versionStringLenBytes)
	}

	newVersion := make([]byte, versionStringLenBytes)
	// golang strings are not NUL terminated, so add one
	suffix += "\x00"
	end := versionStringLenBytes - len(suffix)

	if end < 0 {
		return nil, errors.Errorf("suffix %q is too long for version len: %d", suffix, len(version))
	}

	copy(newVersion, version[0:end])
	copy(newVersion[end:], suffix)

	return newVersion, nil
}

func createRollbackBytes(newRollbackValue uint32) []byte {
	rollbackBytes := make([]byte, rollbackSizeBytes)
	binary.LittleEndian.PutUint32(rollbackBytes, newRollbackValue)
	return rollbackBytes
}

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
