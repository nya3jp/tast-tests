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

func runCommandLine(ctx context.Context, s *testing.State, commandLineArgs []string) ([]byte, error) {
	s.Log("Running command line with arguments: ", commandLineArgs)
	cmd := s.DUT().Conn().CommandContext(ctx, commandLineArgs[0], commandLineArgs[1:]...)
	return cmd.CombinedOutput()
}

func flashromRead(ctx context.Context, s *testing.State, imagePath string) {
	s.Logf("Reading image into file %s", imagePath)

	out, err := runCommandLine(ctx, s, []string{dutFlashromPath, "-r", string(imagePath)})

	testing.ContextLog(ctx, "Flashrom read output:", "\n", string(out))

	if err != nil {
		s.Fatal("Reading image into file failed")
	}

	s.Log("Reading image into file successful")
}

func flashromWrite(ctx context.Context, s *testing.State, imagePath, originalImagePath string, noVerifyAll bool) {
	s.Logf("Writing image from file %s, flash contents %s", imagePath, originalImagePath)

	args := []string{dutFlashromPath, "-w", imagePath, "--flash-contents", originalImagePath}
	if noVerifyAll {
		args = append(args, "--noverify-all")
	}

	out, err := runCommandLine(ctx, s, args)

	testing.ContextLog(ctx, "Flashrom write output:", "\n", string(out))

	if err != nil {
		s.Fatalf("Writing from file %s failed", imagePath)
	}

	s.Logf("Successfully write from file %s, flash contents %s", imagePath, originalImagePath)
}

func getPaddedSection(byteValue byte) [maxSectionSize]byte {
	var data [maxSectionSize]byte
	for i := range data {
		data[i] = byteValue
	}

	return data
}

func createBlobOnDut(ctx context.Context, s *testing.State, byteValue byte, dutBlobFilePath string) {
	testBlobData := getPaddedSection(byteValue)
	if len(testBlobData) == 0 {
		s.Fatal("Empty test data")
	}

	s.Logf("Test blob data with all %v created", byteValue)

	// Create local tmp file
	testBlobFile, err := ioutil.TempFile("", "test_blob.bin")
	if err != nil {
		s.Fatal("Creating test blob file failed")
	}
	defer os.Remove(testBlobFile.Name())

	// Write test data into local tmp file
	err = ioutil.WriteFile(testBlobFile.Name(), testBlobData[:], 0644)
	if err != nil {
		s.Fatal("Writing data to test blob file failed")
	}

	s.Logf("Writing data to test blob file %s successful", testBlobFile.Name())

	// Copy local tmp file to DUT
	fileNamesMap := map[string]string{testBlobFile.Name(): dutBlobFilePath}
	_, err = linuxssh.PutFiles(ctx, s.DUT().Conn(), fileNamesMap, linuxssh.DereferenceSymlinks)
	if err != nil {
		s.Fatalf("Copying test blob file %s to DUT failed", dutBlobFilePath)
	}

	s.Logf("Copying test blob file %s to DUT successful", dutBlobFilePath)
}

func splitByWhitespace(line string) []string {
	trimmedLine := strings.TrimSpace(line)
	trimmedLine = strings.ReplaceAll(trimmedLine, "    ", "")
	trimmedLine = strings.ReplaceAll(trimmedLine, "   ", " ")
	return strings.Split(trimmedLine, " ")
}

func parseAttr(offsetAttr, sizeAttr string) (uint64, uint64, error) {
	offset, err := strconv.ParseUint(strings.ReplaceAll(offsetAttr, "0x", ""), 16, 64)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to parse offset attribute with")
	}
	size, err := strconv.ParseUint(strings.ReplaceAll(sizeAttr, "0x", ""), 16, 64)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to parse offset attribute with")
	}
	return offset, size, nil
}

func getSectionAttributes(ctx context.Context, s *testing.State, imagePath, section string) (uint64, uint64) {
	out, err := runCommandLine(ctx, s, []string{dutDumpFmapPath, imagePath})
	if err != nil {
		s.Fatalf("Dumping fmap from file %s failed", imagePath)
	}

	s.Logf("Dumping fmap from file %s successful", imagePath)

	attributes := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		tokens := splitByWhitespace(line)

		if len(tokens) <= 1 {
			continue
		}

		if tokens[0] == "area_name:" && tokens[1] == section {
			offsetAttr := attributes["area_offset:"]
			sizeAttr := attributes["area_size:"]
			s.Logf("Found section %s in bios file %s with recorded offset %s and size %s", section, imagePath, offsetAttr, sizeAttr)
			offset, size, err := parseAttr(offsetAttr, sizeAttr)
			if err != nil {
				s.Logf("%v", err)
			}
			return offset, size
		}

		attributes[tokens[0]] = tokens[1]
	}

	s.Fatalf("Failed to find section %s in bios file %s", section, imagePath)

	return 0, 0
}

func getInactiveSection(ctx context.Context, s *testing.State) string {
	out, err := runCommandLine(ctx, s, []string{dutCrossystemPath, "mainfw_act"})
	if err != nil {
		s.Fatal("Detecting active section on DUT failed")
	}

	activeSection := string(out)

	section := "Unknown"
	if activeSection == "A" {
		section = "RW_SECTION_B"
	} else if activeSection == "B" {
		section = "RW_SECTION_A"
	} else {
		s.Fatalf("Unexpected active fw %s", activeSection)
	}
	s.Logf("Work section for test (non-active) detected %s", section)

	return section
}

func prepareJunkImage(ctx context.Context, s *testing.State,
	biosImagePath, junkImagePath, ddTemplate string, sectionSize int) {
	// Create junk image, step 1: copy from good bios image
	out, err := runCommandLine(ctx, s, []string{dutCpPath, string(biosImagePath), string(junkImagePath)})

	if err != nil {
		s.Fatal("Copying from good bios image failed")
	}
	s.Logf("Step 1/2 done: successfully copied good bios image from %s to %s", string(biosImagePath), string(junkImagePath))

	// step 2: Set section in the junk image to 'all erased' (all 1s from the blob)
	ddCmd := strings.Split(fmt.Sprintf(ddTemplate+` count=%v`, sectionSize), " ")

	s.Log("ddCmd value ", ddCmd)
	out, err = runCommandLine(ctx, s, ddCmd)

	testing.ContextLog(ctx, "dd command output:", "\n", string(out))

	if err != nil {
		s.Fatal("Set section of the junk image failed")
	}

	s.Log("Step 2/2 done: successfully set section of the junk image with all 1s. Junk image is ready")
}

func FwFlashErasers(ctx context.Context, s *testing.State) {
	// Detect inactive section on DUT
	section := getInactiveSection(ctx, s)

	biosImagePath := dutLocalDataDir + "bios_image.bin"

	// Read the image from chip
	flashromRead(ctx, s, string(biosImagePath))

	// Create a blob of 1s to paste into the image, with maximum size
	// Blob is created locally and then copied into the DUT

	dutBlobFilePath := dutLocalDataDir + "test_blob.bin"
	createBlobOnDut(ctx, s, 0xff, dutBlobFilePath)

	// Find in fmap the AP firmware section which can be overwritten.
	sectionOffset, sectionSize := getSectionAttributes(ctx, s, biosImagePath, section)

	s.Logf("Found offset %v and size %v of section %s", sectionOffset, sectionSize, section)

	// Creating junk image path to be used in the loop
	junkImagePath := dutLocalDataDir + "junk_image.bin"

	// Command to paste the all ones blob into the corrupted image.
	ddTemplate := fmt.Sprintf(`%s if=%s of=%s bs=1 conv=notrunc seek=%v`, dutDdPath, dutBlobFilePath, junkImagePath, sectionOffset)

	s.Logf("ddTemplate value is %s", ddTemplate)

	// Sizes to try to erase.
	sectionSizes := []int{maxSectionSize / 16, maxSectionSize / 8, maxSectionSize / 4, maxSectionSize / 2, maxSectionSize}

	for _, sectionSize := range sectionSizes {
		s.Logf("====== Verifying section of size: %v, preparing junk image ======", sectionSize)

		prepareJunkImage(ctx, s, biosImagePath, junkImagePath, ddTemplate, sectionSize)

		// Now write to chip the corrupted image, this would involve erasing the section of testSize bytes.
		flashromWrite(ctx, s, junkImagePath, biosImagePath, true)

		s.Logf("Successfully write file %s, junk image written to chip", junkImagePath)

		// Now restore the image (write good image back)
		flashromWrite(ctx, s, biosImagePath, junkImagePath, false)

		s.Logf("Successfully write file %s, good bios image is restored on chip", biosImagePath)
	}
}
