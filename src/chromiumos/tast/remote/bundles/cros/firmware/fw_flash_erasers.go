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

	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
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

func getSectionAttributes(ctx context.Context, s *testing.State, biosImagePath, section string) (uint64, uint64) {
	// dump fmap
	out, err := runCommandLine(ctx, s, []string{"/usr/bin/dump_fmap", biosImagePath})

	if err != nil {
		s.Fatalf("Dumping fmap from file %s failed", biosImagePath)
	}

	s.Logf("Dumping fmap from file %s successful", biosImagePath)

	attributes := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		trimmedLine := strings.TrimSpace(line)
		trimmedLine = strings.ReplaceAll(trimmedLine, "    ", "")
		trimmedLine = strings.ReplaceAll(trimmedLine, "   ", " ")
		tokens := strings.Split(trimmedLine, " ")
		if tokens[0] == "area_name:" && tokens[1] == section {
			s.Logf("Found section %s in bios file %s with recorded offset %s and size %s", section, biosImagePath, attributes["area_offset:"], attributes["area_size:"])
			offset, _ := strconv.ParseUint(strings.ReplaceAll(attributes["area_offset:"], "0x", ""), 16, 64)
			size, _ := strconv.ParseUint(strings.ReplaceAll(attributes["area_size:"], "0x", ""), 16, 64)
			return offset, size
		}
		if len(tokens) > 1 {
			attributes[tokens[0]] = tokens[1]
		}
	}

	s.Fatalf("Failed to find section %s in bios file %s", section, biosImagePath)

	return 0, 0
}

func FwFlashErasers(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Detect active section on DUT

	out, err := runCommandLine(ctx, s, []string{"/usr/bin/crossystem", "mainfw_act"})
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

	if err != nil {
		s.Fatal("Detecting active section on DUT failed")
	}

	// Read the image from chip

	dutLocalDataDir := "/usr/local/share/tast/"
	biosImagePath := dutLocalDataDir + "bios_image.bin"

	s.Logf("Reading image into file %s", biosImagePath)

	out, err = runCommandLine(ctx, s, []string{"/usr/sbin/flashrom", "-r", string(biosImagePath)})

	testing.ContextLog(ctx, "Flashrom read output:", "\n", string(out))

	if err != nil {
		s.Fatal("Reading image into file failed")
	}

	s.Log("Reading image into file successful")

	// Create a blob of 1s to paste into the image, with maximum size
	// Blob is created locally and then copied into the DUT

	var testBlobData [4096 * 16]byte // max of test sizes
	for i := range testBlobData {
		testBlobData[i] = 0xff
	}

	s.Log("Test blob data with all 1s created")

	// create local tmp file
	testBlobFile, errF := ioutil.TempFile("", "test_blob.bin")
	if errF != nil {
		s.Fatal("Creating test blob file failed")
	}
	defer os.Remove(testBlobFile.Name()) // cleanup

	s.Logf("Creating test blob file %s successful", testBlobFile.Name())

	// write test data into local tmp file
	err = ioutil.WriteFile(testBlobFile.Name(), testBlobData[:], 0644)
	if err != nil {
		s.Fatal("Writing data to test blob file failed")
	}

	s.Logf("Writing data to test blob file %s successful", testBlobFile.Name())

	// copy local tmp file to DUT

	dutBlobFilePath := dutLocalDataDir + "test_blob.bin"

	_, err = linuxssh.PutFiles(ctx, d.Conn(),
		map[string]string{testBlobFile.Name(): dutBlobFilePath},
		linuxssh.DereferenceSymlinks)
	if err != nil {
		s.Fatalf("Copying test blob file %s to DUT failed", dutBlobFilePath)
	}

	s.Logf("Copying test blob file %s to DUT successful", dutBlobFilePath)

	// Find in fmap the AP firmware section which can be overwritten.
	sectionOffset, sectionSize := getSectionAttributes(ctx, s, biosImagePath, section)

	s.Logf("Found offset %v and size %v of section %s", sectionOffset, sectionSize, section)

	// Creating junk image path to be used in the loop
	junkImagePath := dutLocalDataDir + "junk_image.bin"

	// Command to paste the all ones blob into the corrupted image.
	ddTemplate := fmt.Sprintf(`dd if=%s of=%s bs=1 conv=notrunc seek=%v`, dutBlobFilePath, junkImagePath, sectionOffset)

	s.Logf("ddTemplate value is %s", ddTemplate)

	// Sizes to try to erase.
	testSizes := []int{4096, 4096 * 2, 4096 * 4, 4096 * 8, 4096 * 16}

	for _, testSize := range testSizes {
		s.Logf("Verifying section of size: %v, preparing junk image", testSize)

		// Create junk image, step 1: copy from good bios image
		out, err = runCommandLine(ctx, s, []string{"cp", string(biosImagePath), string(junkImagePath)})

		if err != nil {
			s.Fatal("Copying from good bios image failed")
		}
		s.Logf("Step 1, successfully copied good bios image from %s to %s", string(biosImagePath), string(junkImagePath))

		// step 2: Set section in the junk image to 'all erased' (all 1s from the blob)
		ddCmd := strings.Split(fmt.Sprintf(ddTemplate+` count=%v`, testSize), " ")

		s.Log("ddCmd value ", ddCmd)
		out, err = runCommandLine(ctx, s, ddCmd)

		testing.ContextLog(ctx, "dd command output:", "\n", string(out))

		if err != nil {
			s.Fatal("Set section of the junk image failed")
		}

		s.Log("Step 2: successfully set section of the junk image with all 1s. Junk image is ready")

		// Now write to chip the corrupted image, this would involve erasing the section of testSize bytes.
		out, err = runCommandLine(ctx, s, []string{"/usr/sbin/flashrom", "-w", junkImagePath, "--flash-contents", biosImagePath, "--noverify-all"})

		testing.ContextLog(ctx, "Flashrom write junk file output:", "\n", string(out))

		if err != nil {
			s.Fatalf("Writing file %s failed", junkImagePath)
		}

		s.Logf("Successfully write file %s, junk image written to chip", junkImagePath)

		// Now restore the image (write good image back)
		out, err = runCommandLine(ctx, s, []string{"/usr/sbin/flashrom", "-w", biosImagePath, "--flash-contents", junkImagePath})

		testing.ContextLog(ctx, "Flashrom write good image file output:", "\n", string(out))

		if err != nil {
			s.Fatalf("Writing file %s failed", biosImagePath)
		}

		s.Logf("Successfully write file %s, good bios image is restored on chip", biosImagePath)
	}
}
