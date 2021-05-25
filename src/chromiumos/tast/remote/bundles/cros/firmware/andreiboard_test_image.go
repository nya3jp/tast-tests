// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/dut"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AndreiboardTestImage,
		Desc:         "Test wrapper for running test images on Andreiboard",
		Timeout:      8 * time.Minute,
		Vars:         []string{"image", "spiflash"},
		Contacts:     []string{"cros-fw-engprod@google.com", "aluo@google.com"},
		Attr:         []string{"group:firmware"},
	})
}

func copyFileToDut(ctx context.Context, d *dut.DUT, localFile string, remoteFile string) error {
    _, err := linuxssh.PutFiles(ctx, d.Conn(), map[string]string{localFile: remoteFile}, linuxssh.DereferenceSymlinks)
    return err
}

func AndreiboardTestImage(ctx context.Context, s *testing.State) {
    remoteBoard := true

    image := s.RequiredVar("image")
    spiflash, _ := s.Var("spiflash")

    if spiflash == "" {
        spiflash = "/mnt/host/source/src/platform/cr50-utils/software/tools/SPI/spiflash"
    }
    s.Logf("Using image: %v", image)
    s.Logf("Using spiflash binary: %v", spiflash)

    cmd := testexec.CommandContext(ctx, "ls", "-l", image)
    out, err := cmd.Output()
    if err == nil {
        s.Logf("ls image output: %v", string(out))
    }

    if remoteBoard {
        if err := copyFileToDut(ctx, s.DUT(), image, "/tmp/full_image.signed"); err != nil {
            s.Fatalf("Copy image failed: %v", err)
        }

        if err := copyFileToDut(ctx, s.DUT(), spiflash, "/tmp/spiflash"); err != nil {
            s.Fatalf("Copy spiflash failed: %v", err)
        }
	s.Log("Image and spiflash copied to dut.")
    }
}
