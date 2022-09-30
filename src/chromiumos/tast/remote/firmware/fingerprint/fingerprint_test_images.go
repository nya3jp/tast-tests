// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"bytes"
	"context"
	"encoding/binary"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	fp "chromiumos/tast/common/fingerprint"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

const (
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

var devKeyMap = map[fp.BoardName]string{
	fp.BoardNameBloonchipper: BloonchipperDevKey,
	fp.BoardNameDartmonkey:   DartmonkeyDevKey,
	fp.BoardNameNami:         NamiFPDevKey,
	fp.BoardNameNocturne:     NocturneFPDevKey,
}

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

// FMAPSection describes a firmware map section.
type FMAPSection string

const (
	// ROFirmwareID is the read-only firmware ID.
	ROFirmwareID FMAPSection = "RO_FRID"
	// RWFirmwareID is the read-write firmware ID.
	RWFirmwareID FMAPSection = "RW_FWID"
	// RWRollbackVersion is the read-write rollback version.
	RWRollbackVersion FMAPSection = "RW_RBVER"
	// RWFirmware is the read-write section.
	RWFirmware FMAPSection = "EC_RW"
	// SignatureRW is the signature section.
	SignatureRW FMAPSection = "SIG_RW"
)

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

type fmapSection struct {
	Name   string
	Offset int
	Size   int
}

type fmapSectionValue struct {
	Section *fmapSection
	Bytes   []byte
}

type keyPair struct {
	PublicKeyPath  string
	PrivateKeyPath string
}

type firmwareImageGenerator struct {
	devKeyPair           *keyPair
	futilityPath         string
	origFirmwareFilePath string
	rwVersion            *fmapSectionValue
	roVersion            *fmapSectionValue
}

// DevKeyForFPBoard gets the dev key for the given fpBoard.
func DevKeyForFPBoard(fpBoard fp.BoardName) string {
	return devKeyMap[fpBoard]
}

func hostCommand(ctx context.Context, name string, arg ...string) *exec.Cmd {
	testing.ContextLogf(ctx, "Command: %s %s", name, strings.Join(arg, " "))
	return exec.CommandContext(ctx, name, arg...)
}

func signFirmware(ctx context.Context, futilityPath, privateKeyFile, firmwareFile string) error {
	cmd := []string{futilityPath, "sign", "--type", "rwsig", "--prikey", privateKeyFile, "--version", "1", firmwareFile}
	if err := hostCommand(ctx, cmd[0], cmd[1:]...).Run(); err != nil {
		return errors.Wrap(err, "failed to run futility sign")
	}
	return nil
}

func createKeyPairFromRSAKey(ctx context.Context, futilityPath, pemFilePath, keyDescription string) (*keyPair, error) {
	curDir, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get working directory")
	}

	cmd := []string{futilityPath, "create", "--desc", keyDescription, pemFilePath, "key"}
	if err := hostCommand(ctx, cmd[0], cmd[1:]...).Run(); err != nil {
		return nil, errors.Wrap(err, "failed to run futility create")
	}

	return &keyPair{
		PublicKeyPath:  filepath.Join(curDir, "key.vbpubk2"),
		PrivateKeyPath: filepath.Join(curDir, "key.vbprik2"),
	}, nil
}

func fmapSectionInfo(ctx context.Context, futilityPath, firmwareFilePath string, section FMAPSection) (*fmapSection, error) {
	cmd := []string{futilityPath, "dump_fmap", "-p", firmwareFilePath, string(section)}
	output, err := hostCommand(ctx, cmd[0], cmd[1:]...).Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run futility dump_fmap")
	}

	// The format of the output is:
	// SECTION OFFSET SIZE
	fields := strings.Fields(string(output))
	if len(fields) != 3 {
		return nil, errors.Errorf("unexpected number of fields: %q, output: %q", len(fields), string(output))
	}

	name := fields[0]

	offset, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert offset string to int")
	}

	size, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert size string to int")
	}

	return &fmapSection{
		Name:   name,
		Offset: offset,
		Size:   size,
	}, nil
}

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
	// Check for first null terminator and use that as end.
	idx := bytes.Index(version, []byte("\x00"))
	if idx > -1 && idx < end {
		diff := end - idx
		end = idx
		for i := 0; i < diff; i++ {
			suffix += "\x00"
		}
	}

	copy(newVersion, version[0:end])
	copy(newVersion[end:], suffix)

	return newVersion, nil
}

func addSuffixToVersionString(filePath, suffix string, version *fmapSectionValue) error {
	newVersion, err := createVersionStringWithSuffix(suffix, version.Bytes)
	if err != nil {
		return errors.Wrap(err, "failed to modify version string")
	}

	if err := writeFileAtOffset(filePath, newVersion, int64(version.Section.Offset)); err != nil {
		return errors.Wrap(err, "failed to update RO version string")
	}

	return nil
}

func createRollbackBytes(newRollbackValue uint32) []byte {
	rollbackBytes := make([]byte, rollbackSizeBytes)
	binary.LittleEndian.PutUint32(rollbackBytes, newRollbackValue)
	return rollbackBytes
}

func modifyFirmwareFileRollbackValue(firmwareFilePath string, newRollbackValue uint32, rollback *fmapSectionValue) error {
	if rollback.Section.Size != rollbackSizeBytes {
		return errors.Errorf("incorrect version size, actual: %v, expected: %v", rollback.Section.Size, rollbackSizeBytes)
	}

	rollbackBytes := createRollbackBytes(newRollbackValue)

	if err := writeFileAtOffset(firmwareFilePath, rollbackBytes, int64(rollback.Section.Offset)); err != nil {
		return errors.Wrap(err, "failed to update rollback")
	}

	return nil
}

func newFirmwareImageGenerator(devKeyPair *keyPair, futilityPath, origFirmwareFilePath string, rwVersion, roVersion *fmapSectionValue) *firmwareImageGenerator {
	return &firmwareImageGenerator{
		devKeyPair:           devKeyPair,
		futilityPath:         futilityPath,
		origFirmwareFilePath: origFirmwareFilePath,
		rwVersion:            rwVersion,
		roVersion:            roVersion,
	}
}

func (f *firmwareImageGenerator) DevSignedImage(ctx context.Context) (string, error) {
	devFilePath := strings.TrimSuffix(f.origFirmwareFilePath, filepath.Ext(f.origFirmwareFilePath)) + ".dev"

	if err := fsutil.CopyFile(f.origFirmwareFilePath, devFilePath); err != nil {
		return "", errors.Wrap(err, "failed to copy file")
	}

	if err := addSuffixToVersionString(devFilePath, ".dev", f.roVersion); err != nil {
		return "", errors.Wrap(err, "failed to modify RO version string")
	}

	if err := addSuffixToVersionString(devFilePath, ".dev", f.rwVersion); err != nil {
		return "", errors.Wrap(err, "failed to modify RW version string")
	}

	// The firmware was modified, so we need to re-sign it.
	if err := signFirmware(ctx, f.futilityPath, f.devKeyPair.PrivateKeyPath, devFilePath); err != nil {
		return "", errors.Wrap(err, "failed to sign firmware")
	}

	return devFilePath, nil
}

func (f *firmwareImageGenerator) Rollback(ctx context.Context, rollback *fmapSectionValue, newRollbackValue uint32) (string, error) {
	versionSuffix := ".rb" + strconv.FormatUint(uint64(newRollbackValue), 10)
	ext := ".dev" + versionSuffix
	rollbackFilePath := strings.TrimSuffix(f.origFirmwareFilePath, filepath.Ext(f.origFirmwareFilePath)) + ext

	if err := fsutil.CopyFile(f.origFirmwareFilePath, rollbackFilePath); err != nil {
		return "", errors.Wrap(err, "failed to copy file")
	}

	if err := addSuffixToVersionString(rollbackFilePath, ".dev", f.roVersion); err != nil {
		return "", errors.Wrap(err, "failed to modify RO version string")
	}

	if err := addSuffixToVersionString(rollbackFilePath, versionSuffix, f.rwVersion); err != nil {
		return "", errors.Wrap(err, "failed to modify RW version string")
	}

	if err := modifyFirmwareFileRollbackValue(rollbackFilePath, newRollbackValue, rollback); err != nil {
		return "", errors.Wrap(err, "failed to modify rollback value")
	}

	// The firmware was modified, so we need to re-sign it
	if err := signFirmware(ctx, f.futilityPath, f.devKeyPair.PrivateKeyPath, rollbackFilePath); err != nil {
		return "", errors.Wrap(err, "failed to sign firmware")
	}

	return rollbackFilePath, nil
}

func (f *firmwareImageGenerator) CorruptFirstByte(ctx context.Context) (string, error) {
	corruptFilePath := strings.TrimSuffix(f.origFirmwareFilePath, filepath.Ext(f.origFirmwareFilePath)) + "_corrupt_first_byte.bin"

	if err := fsutil.CopyFile(f.origFirmwareFilePath, corruptFilePath); err != nil {
		return "", errors.Wrap(err, "failed to copy file")
	}

	rwSection, err := fmapSectionInfo(ctx, f.futilityPath, corruptFilePath, RWFirmware)
	if err != nil {
		return "", errors.Wrap(err, "failed to get FMAP info for EC_RW")
	}

	byteToCorrupt := make([]byte, 1)
	if err := readFileAtOffset(corruptFilePath, byteToCorrupt, int64(rwSection.Offset)+100); err != nil {
		return "", errors.Wrap(err, "failed to read byte")
	}

	byteToCorrupt[0]++
	if err := writeFileAtOffset(corruptFilePath, byteToCorrupt, int64(rwSection.Offset)+100); err != nil {
		return "", errors.Wrap(err, "failed to write corrupted byte")
	}

	return corruptFilePath, nil
}

func (f *firmwareImageGenerator) CorruptLastByte(ctx context.Context) (string, error) {
	corruptFilePath := strings.TrimSuffix(f.origFirmwareFilePath, filepath.Ext(f.origFirmwareFilePath)) + "_corrupt_last_byte.bin"

	if err := fsutil.CopyFile(f.origFirmwareFilePath, corruptFilePath); err != nil {
		return "", errors.Wrap(err, "failed to copy file")
	}

	rwSection, err := fmapSectionInfo(ctx, f.futilityPath, corruptFilePath, SignatureRW)
	if err != nil {
		return "", errors.Wrap(err, "failed to get FMAP info for SIG_RW")
	}

	byteToCorrupt := make([]byte, 1)
	if err := readFileAtOffset(corruptFilePath, byteToCorrupt, int64(rwSection.Offset)-100); err != nil {
		return "", errors.Wrap(err, "failed to read byte")
	}

	byteToCorrupt[0]++
	if err := writeFileAtOffset(corruptFilePath, byteToCorrupt, int64(rwSection.Offset)-100); err != nil {
		return "", errors.Wrap(err, "failed to write corrupted byte")
	}

	return corruptFilePath, nil
}

func readFMAPSection(ctx context.Context, futilityPath, firmwareFilePath string, section FMAPSection) (*fmapSectionValue, error) {
	sectionInfo, err := fmapSectionInfo(ctx, futilityPath, firmwareFilePath, section)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get FMAP info for section: %q", section)
	}

	sectionData := make([]byte, sectionInfo.Size)
	if err := readFileAtOffset(firmwareFilePath, sectionData, int64(sectionInfo.Offset)); err != nil {
		return nil, errors.Wrapf(err, "unable to read FMAP section: %q", section)
	}

	return &fmapSectionValue{
		Section: sectionInfo,
		Bytes:   sectionData,
	}, nil
}

// GenerateTestFirmwareImages generates a set of test firmware images from the firmware that is on the DUT.
func GenerateTestFirmwareImages(ctx context.Context, d *rpcdut.RPCDUT, futilityPath, keyFilePath string, fpBoard fp.BoardName, buildFWFile, dutTempDir string) (ret TestImages, retErr error) {
	testing.ContextLog(ctx, "Creating temp dir")
	serverTmpDir, err := ioutil.TempDir("", "*")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(serverTmpDir)

	testing.ContextLog(ctx, "Copying firmware from DUT to host")
	serverFWFilePath := filepath.Join(serverTmpDir, filepath.Base(buildFWFile))
	if err := linuxssh.GetFile(ctx, d.Conn(), buildFWFile, serverFWFilePath, linuxssh.DereferenceSymlinks); err != nil {
		return nil, errors.Wrap(err, "failed to get file")
	}

	origFWFileCopy := filepath.Join(serverTmpDir, string(fpBoard)+".bin")
	if err := fsutil.CopyFile(serverFWFilePath, origFWFileCopy); err != nil {
		return nil, errors.Wrap(err, "failed to copy original firmware file")
	}

	devKeyPair, err := createKeyPairFromRSAKey(ctx, futilityPath, keyFilePath, string(fpBoard)+" dev key")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create key pair")
	}

	roVersion, err := readFMAPSection(ctx, futilityPath, origFWFileCopy, ROFirmwareID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read RO firmware info")
	}

	rwVersion, err := readFMAPSection(ctx, futilityPath, origFWFileCopy, RWFirmwareID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read RW firmware info")
	}

	rollback, err := readFMAPSection(ctx, futilityPath, origFWFileCopy, RWRollbackVersion)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read rollback version")
	}

	firmwareImageGenerator := newFirmwareImageGenerator(devKeyPair, futilityPath, origFWFileCopy, roVersion, rwVersion)

	devFilePath, err := firmwareImageGenerator.DevSignedImage(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate dev signed image")
	}

	rollbackZeroFilePath, err := firmwareImageGenerator.Rollback(ctx, rollback, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate image with modified rollback value 0")
	}

	rollbackOneFilePath, err := firmwareImageGenerator.Rollback(ctx, rollback, 1)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate image with modified rollback value 1")
	}

	rollbackNineFilePath, err := firmwareImageGenerator.Rollback(ctx, rollback, 9)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate image with modified rollback value 9")
	}

	corruptFirstBytePath, err := firmwareImageGenerator.CorruptFirstByte(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate image with corrupt first byte")
	}

	corruptLastBytePath, err := firmwareImageGenerator.CorruptLastByte(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate image with corrupt last byte")
	}

	images := TestImages{
		TestImageTypeOriginal:         &TestImageData{Path: origFWFileCopy},
		TestImageTypeDev:              &TestImageData{Path: devFilePath},
		TestImageTypeCorruptFirstByte: &TestImageData{Path: corruptFirstBytePath},
		TestImageTypeCorruptLastByte:  &TestImageData{Path: corruptLastBytePath},
		TestImageTypeDevRollbackZero:  &TestImageData{Path: rollbackZeroFilePath},
		TestImageTypeDevRollbackOne:   &TestImageData{Path: rollbackOneFilePath},
		TestImageTypeDevRollbackNine:  &TestImageData{Path: rollbackNineFilePath},
	}

	filesToCopy := make(map[string]string)
	for imageType, imageData := range images {
		dutFileName := filepath.Join(dutTempDir, generatedImagesSubDirectory, filepath.Base(imageData.Path))
		filesToCopy[imageData.Path] = dutFileName
		images[imageType].Path = dutFileName
	}

	testing.ContextLog(ctx, "Copying generated firmware images to DUT")
	if _, err := linuxssh.PutFiles(ctx, d.Conn(), filesToCopy, linuxssh.PreserveSymlinks); err != nil {
		return nil, errors.Wrapf(err, "failed to copy files from %q to %q", serverTmpDir, dutTempDir)
	}

	for _, imageData := range images {
		// Make sure that images were actually copied to DUT.
		exists, err := dutfs.NewClient(d.RPC().Conn).Exists(ctx, imageData.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to check existence of %q", imageData.Path)
		}
		if !exists {
			return nil, errors.Errorf("expected file to exist, but it does not: %q", imageData.Path)
		}

		// Collect the version strings from each of the generated images.
		version, err := GetBuildROFirmwareVersion(ctx, d, imageData.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get RO version from firmware file: %q", imageData.Path)
		}
		imageData.ROVersion = version

		version, err = GetBuildRWFirmwareVersion(ctx, d, imageData.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get RW version from firmware file: %q", imageData.Path)
		}
		imageData.RWVersion = version
	}

	return images, nil
}
