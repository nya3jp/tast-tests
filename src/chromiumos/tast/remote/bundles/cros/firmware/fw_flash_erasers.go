// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

const (
	// Directory on the DUT where all test files are created.
	dutLocalDataDir = "/usr/local/share/tast/"

	// Full path to flashrom binary on DUT.
	dutFlashromPath = "/usr/sbin/flashrom"

	// Full path to crossystem on DUT.
	dutCrossystemPath = "/usr/bin/crossystem"

	// Full path to dump_fmap on DUT.
	dutDumpFmapPath = "/usr/bin/dump_fmap"

	// Full path to dd on DUT.
	dutDdPath = "/bin/dd"

	// Full path to cp on DUT.
	dutCpPath = "/bin/cp"

	// Maximum size of the section to erase.
	maxSectionSize = 4096 * 16
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FwFlashErasers,
		Desc:         "Test erase functions by calling flashrom to erase and write blocks of different sizes",
		Contacts:     []string{"aklm@chromium.org"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		SoftwareDeps: []string{"flashrom"},
	})
}

func sectionSizes() []int {
	return []int{maxSectionSize / 16, maxSectionSize / 8, maxSectionSize / 4, maxSectionSize / 2, maxSectionSize}
}

func runCommandLine(ctx context.Context, conn *ssh.Conn, args []string, logOutput bool) ([]byte, error) {
	testing.ContextLog(ctx, "Running command line with arguments: ", args)
	cmd := conn.CommandContext(ctx, args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if logOutput {
		testing.ContextLog(ctx, "Command line output:", "\n", string(out))
	}
	return out, err
}

func flashromRead(ctx context.Context, conn *ssh.Conn, imagePath string) error {
	testing.ContextLogf(ctx, "Reading image into file %s", imagePath)

	_, err := runCommandLine(ctx, conn, []string{dutFlashromPath, "-r", imagePath}, true)
	return err
}

func flashromWrite(ctx context.Context, conn *ssh.Conn, imagePath, originalImagePath string, noVerifyAll bool) error {
	testing.ContextLogf(ctx, "Writing image from file %s, flash contents %s", imagePath, originalImagePath)

	args := []string{dutFlashromPath, "-w", imagePath, "--flash-contents", originalImagePath}
	if noVerifyAll {
		args = append(args, "--noverify-all")
	}

	_, err := runCommandLine(ctx, conn, args, true)
	return err
}

func paddedSection(byteValue byte) [maxSectionSize]byte {
	var data [maxSectionSize]byte
	for i := range data {
		data[i] = byteValue
	}

	return data
}

func createBlobOnDut(ctx context.Context, conn *ssh.Conn, byteValue byte, dutBlobFilePath string) error {
	testBlobData := paddedSection(byteValue)
	if len(testBlobData) == 0 {
		return errors.New("empty test data")
	}

	testing.ContextLogf(ctx, "Test blob data with all %v created", byteValue)

	// Create local tmp file
	testBlobFile, err := ioutil.TempFile("", "test_blob.bin")
	if err != nil {
		return errors.New("creating test blob file failed")
	}
	defer os.Remove(testBlobFile.Name())

	// Write test data into local tmp file
	if err = ioutil.WriteFile(testBlobFile.Name(), testBlobData[:], 0644); err != nil {
		return errors.New("writing data to test blob file failed")
	}

	testing.ContextLogf(ctx, "Writing data to test blob file %s successful", testBlobFile.Name())

	// Copy local tmp file to DUT
	fileNamesMap := map[string]string{testBlobFile.Name(): dutBlobFilePath}
	if _, err := linuxssh.PutFiles(ctx, conn, fileNamesMap, linuxssh.DereferenceSymlinks); err != nil {
		return errors.Errorf("copying test blob file %s to DUT failed", dutBlobFilePath)
	}

	testBlobFile.Close()
	testing.ContextLogf(ctx, "Copying test blob file %s to DUT successful", dutBlobFilePath)
	return nil
}

func parseAttr(offsetAttr, sizeAttr string) (offset, size uint64, err error) {
	// Attributes are in hex with 0x prefix
	offset, err = strconv.ParseUint(offsetAttr, 0, 64)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to parse offset attribute with")
	}
	size, err = strconv.ParseUint(sizeAttr, 0, 64)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to parse size attribute with")
	}
	return offset, size, nil
}

func sectionAttributes(ctx context.Context, conn *ssh.Conn, imagePath, section string) (offset, size uint64, err error) {
	out, err := runCommandLine(ctx, conn, []string{dutDumpFmapPath, imagePath}, false)
	if err != nil {
		return 0, 0, errors.Errorf("dumping fmap from file %s failed", imagePath)
	}

	testing.ContextLogf(ctx, "Dumping fmap from file %s successful", imagePath)

	attributes := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		tokens := strings.Fields(line)

		if len(tokens) <= 1 {
			continue
		}

		if tokens[0] == "area_name:" && tokens[1] == section {
			offsetAttr := attributes["area_offset:"]
			sizeAttr := attributes["area_size:"]
			testing.ContextLogf(ctx, "Found section %s in bios file %s with recorded offset %s and size %s", section, imagePath, offsetAttr, sizeAttr)
			offset, size, err := parseAttr(offsetAttr, sizeAttr)
			if err != nil {
				return 0, 0, err
			}
			return offset, size, nil
		}

		attributes[tokens[0]] = tokens[1]
	}

	return 0, 0, errors.Errorf("failed to find section %s in bios file %s", section, imagePath)
}

func inactiveSection(ctx context.Context, conn *ssh.Conn) (string, error) {
	out, err := runCommandLine(ctx, conn, []string{dutCrossystemPath, "mainfw_act"}, false)
	if err != nil {
		return "Unknown", errors.New("detecting active section on DUT failed")
	}

	activeSection := string(out)

	section := "Unknown"
	switch activeSection {
	case "A":
		section = "RW_SECTION_B"
	case "B":
		section = "RW_SECTION_A"
	default:
		return "Unknown", errors.Errorf("unexpected active fw %s", activeSection)
	}
	testing.ContextLogf(ctx, "Work section for test (non-active) detected %s", section)

	return section, nil
}

func prepareJunkImage(ctx context.Context, conn *ssh.Conn,
	biosImagePath, junkImagePath string, ddCmd []string, sectionSize int) error {
	// Create junk image, step 1: copy from good bios image
	if _, err := runCommandLine(ctx, conn, []string{dutCpPath, biosImagePath, junkImagePath}, false); err != nil {
		return errors.New("copying from good bios image failed")
	}
	testing.ContextLogf(ctx, "Step 1/2 done: successfully copied good bios image from %s to %s", biosImagePath, junkImagePath)

	// step 2: Set section in the junk image to 'all erased' (all 1s from the blob)
	ddCmd = append(ddCmd, fmt.Sprintf("count=%v", sectionSize))

	testing.ContextLog(ctx, "ddCmd value ", ddCmd)
	if _, err := runCommandLine(ctx, conn, ddCmd, true); err != nil {
		return errors.New("set section of the junk image failed")
	}

	testing.ContextLog(ctx, "Step 2/2 done: successfully set section of the junk image with all 1s. Junk image is ready")
	return nil
}

func FwFlashErasers(ctx context.Context, s *testing.State) {
	// Detect inactive section on DUT
	section, err := inactiveSection(ctx, s.DUT().Conn())
	if err != nil {
		s.Fatal("Detect inactive section failed: ", err)
	}

	biosImagePath := dutLocalDataDir + "bios_image.bin"

	// Read the image from chip
	if err := flashromRead(ctx, s.DUT().Conn(), string(biosImagePath)); err != nil {
		s.Fatalf("Flashrom read into file %s failed: %v", string(biosImagePath), err)
	}

	// Create a blob of 1s to paste into the image, with maximum size
	// Blob is created locally and then copied into the DUT

	dutBlobFilePath := dutLocalDataDir + "test_blob.bin"
	if err := createBlobOnDut(ctx, s.DUT().Conn(), 0xff, dutBlobFilePath); err != nil {
		s.Fatal("Create blob of 1s on DUT failed: ", err)
	}

	// Find in fmap the AP firmware section which can be overwritten.
	sectionOffset, sectionSize, err := sectionAttributes(ctx, s.DUT().Conn(), biosImagePath, section)
	if err != nil {
		s.Fatal("Find section attributes in fmap failed: ", err)
	}

	s.Logf("Found offset %v and size %v of section %s", sectionOffset, sectionSize, section)

	// Creating junk image path to be used in the loop
	junkImagePath := dutLocalDataDir + "junk_image.bin"

	// Command line args to paste the all ones blob into the corrupted image.
	ddArgs := []string{dutDdPath, fmt.Sprintf("if=%s", dutBlobFilePath), fmt.Sprintf("of=%s", junkImagePath),
		"bs=1", "conv=notrunc", fmt.Sprintf("seek=%v", sectionOffset)}

	s.Log("ddArgs value is ", ddArgs)

	// Sizes to try to erase.
	sizes := sectionSizes()

	for _, sectionSize := range sizes {
		s.Logf("====== Verifying section of size: %v, preparing junk image ======", sectionSize)

		if err := prepareJunkImage(ctx, s.DUT().Conn(), biosImagePath, junkImagePath, ddArgs, sectionSize); err != nil {
			s.Fatal("Prepare junk image failed: ", err)
		}

		// Now write to chip the corrupted image, this would involve erasing the section of testSize bytes.
		if err := flashromWrite(ctx, s.DUT().Conn(), junkImagePath, biosImagePath, true); err != nil {
			s.Fatalf("Flashrom write from file %s failed: %v", junkImagePath, err)
		}

		s.Logf("Successfully write file %s, junk image written to chip", junkImagePath)

		// Now restore the image (write good image back)
		if err := flashromWrite(ctx, s.DUT().Conn(), biosImagePath, junkImagePath, false); err != nil {
			s.Fatalf("Flashrom write from file %s failed: %v", biosImagePath, err)
		}

		s.Logf("Successfully write file %s, good bios image is restored on chip", biosImagePath)
	}
}
