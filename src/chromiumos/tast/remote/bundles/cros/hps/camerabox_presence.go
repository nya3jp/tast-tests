// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/camera/chart"
	"chromiumos/tast/common/hps/hpsutil"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraboxPresence,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the HPS can correctly detect presence from another tablet",
		Data: []string{hpsutil.PersonPresentPageArchiveFilename,
			hpsutil.P2PowerCycleFilename},
		Contacts: []string{
			"eunicesun@google.com",
			"mblsha@google.com",
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:camerabox", "group:hps"},
		Timeout:      20 * time.Minute,
		SoftwareDeps: []string{"hps", "chrome", caps.BuiltinCamera},
		Vars:         []string{"tablet"},
	})
}

func CameraboxPresence(ctx context.Context, s *testing.State) {
	// Connecting to DUT with HPS and send powercycle file to it
	d := s.DUT()
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Power-cycle HPS after two people are already visible on the screen so that
	// the HPS would already be able to correctly adjust the exposure right from the start.

	// Sending powercycle python file to DUT with HPS
	powercycleTmpDir, err := d.Conn().CommandContext(ctx, "mktemp", "-d", "/tmp/powercycle_XXXXX").Output()
	if err != nil {
		s.Fatal("Failed to create test directory under /tmp for putting powercycle file: ", err)
	}
	powercycleDirPath := strings.TrimSpace(string(powercycleTmpDir))
	powercycleFilePath := filepath.Join(powercycleDirPath, hpsutil.P2PowerCycleFilename)
	defer d.Conn().CommandContext(ctx, "rm", "-r", powercycleDirPath).Output()
	if _, err := linuxssh.PutFiles(
		ctx, d.Conn(), map[string]string{
			s.DataPath(hpsutil.P2PowerCycleFilename): powercycleFilePath,
		},
		linuxssh.DereferenceSymlinks); err != nil {
		s.Fatalf("Failed to send data to remote data path %v: %v", powercycleFilePath, err)
	}
	testing.ContextLog(ctx, "Sending file to dut, path being: ", powercycleFilePath)

	// Extract files from tar
	archive := s.DataPath(hpsutil.PersonPresentPageArchiveFilename)
	dirPath := filepath.Dir(archive)
	testing.ContextLog(ctx, "dirpath: ", dirPath)

	tarOut, err := testexec.CommandContext(ctx, "tar", "--strip-components=1", "-xvf", archive, "-C", dirPath).Output()
	testing.ContextLog(ctx, "Extracting following files: ", string(tarOut))
	if err != nil {
		s.Fatal("Failed to untar test artifacts: ", err)
	}

	// Creating hps context
	hctx, err := hpsutil.NewHpsContext(ctx, powercycleFilePath, hpsutil.DeviceTypeBuiltin, s.OutDir(), d.Conn())
	if err != nil {
		s.Fatal("Error creating HpsContext: ", err)
	}

	// Connecting to the other tablet that will render the picture
	var chartAddr string
	if altAddr, ok := s.Var("tablet"); ok {
		chartAddr = altAddr
	}

	picture := filepath.Join(dirPath, "IMG_7451.jpg")
	chartPaths := []string{
		filepath.Join(dirPath, "no-person-present.html"),
		filepath.Join(dirPath, "person-present.html"),
		filepath.Join(dirPath, "two-people-present.html")}
	filePaths := append(chartPaths, picture)

	c, hostPaths, err := chart.New(ctx, d, chartAddr, s.OutDir(), filePaths)
	if err != nil {
		s.Fatal("Put picture failed: ", err)
	}
	testing.ContextLog(ctx, "hostPaths: ", hostPaths)

	for i := 0; i < 2; i++ {
		// for no person
		c.Display(ctx, hostPaths[0])
		if err := numPersonDetect(hctx, strconv.Itoa(i)); err != nil {
			s.Fatal("Failed to run N presence ops: ", err)
		}

		// for one/two person present
		c.Display(ctx, hostPaths[i+1])
		if err := numPersonDetect(hctx, strconv.Itoa(i)); err != nil {
			s.Fatal("Failed to run N presence ops: ", err)
		}
	}
}

func numPersonDetect(hctx *hpsutil.HpsContext, feature string) error {

	if _, err := hpsutil.EnablePresence(hctx, feature); err != nil {
		return errors.Wrap(err, "enablePresence failed")
	}

	if _, err := hpsutil.WaitForNPresenceOps(hctx, hpsutil.WaitNOpsBeforeStart, feature); err != nil {
		return errors.Wrap(err, "failed to run N presence ops")
	}

	_, err := hpsutil.WaitForNPresenceOps(hctx, hpsutil.GetNOpsToVerifyPresenceWorks, feature)
	if err != nil {
		return errors.Wrap(err, "failed to run N presence ops")
	}
	return nil
}
