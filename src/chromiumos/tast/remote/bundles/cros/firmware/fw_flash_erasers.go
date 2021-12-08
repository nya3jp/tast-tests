// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"os"

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

func FwFlashErasers(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// === temp code for dev purposes, to be removed before sending for review
	/*	s.Log("Hello World from FlashErasers");

		probeCmd := []string{"/usr/sbin/flashrom", "-p", "internal", "-V"}
		cmdP := d.Conn().CommandContext(ctx, probeCmd[0], probeCmd[1:]...)
		outP, errP := cmdP.CombinedOutput()

		testing.ContextLog(ctx, "Flashrom probe output:", "\n", string(outP))

		if errP != nil {
			s.Fatal("Flashrom probe failed")
		}
	*/ // === end of temp code

	// Detect active section on DUT

	commandLine := []string{"/usr/bin/crossystem", "mainfw_act"}
	cmd := d.Conn().CommandContext(ctx, commandLine[0], commandLine[1:]...)
	out, err := cmd.CombinedOutput()
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
	biosImage := dutLocalDataDir + "bios_image.bin"

	s.Logf("Reading image into file %s", biosImage)

	commandLine = []string{"/usr/sbin/flashrom", "-r", string(biosImage)}
	cmd = d.Conn().CommandContext(ctx, commandLine[0], commandLine[1:]...)
	out, err = cmd.CombinedOutput()

	testing.ContextLog(ctx, "Flashrom read output:", "\n", string(out))

	if err != nil {
		s.Fatal("Reading image into file failed")
	}

	s.Log("Reading image into file successful")

	// Sizes to try to erase

	//testSizes := []int{4096, 4096 * 2, 4096 * 4, 4096 * 8, 4096 * 16}

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

	dutBlobFile := dutLocalDataDir + "test_blob.bin"

	_, err = linuxssh.PutFiles(ctx, d.Conn(),
		map[string]string{testBlobFile.Name(): dutBlobFile},
		linuxssh.DereferenceSymlinks)
	if err != nil {
		s.Fatalf("Copying test blob file %s to DUT failed", dutBlobFile)
	}

	s.Logf("Copying test blob file %s to DUT successful", dutBlobFile)
}
