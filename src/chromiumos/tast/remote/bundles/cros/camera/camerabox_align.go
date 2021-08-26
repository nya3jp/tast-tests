// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/remote/bundles/cros/camera/chart"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraboxAlign,
		Desc:         "Verifying alignment of chart tablet screen and target facing camera FOV in camerabox setup",
		Data:         []string{"camerabox_align.svg", "camerabox_align.html", "camerabox_align.css", "camerabox_align.js"},
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:camerabox"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.camerabox.AlignmentService"},
		Vars:         []string{"chart", "facing", "user", "pass"},
		Params: []testing.Param{
			// Manual test for interactively guiding lab eng to align the camerabox setup.
			testing.Param{
				Timeout: 20 * time.Minute,
				// Facing is specified from -var=facing=.
				Val: pb.Facing_FACING_UNSET,
			},
			// Checks alignment for every regression test run.
			testing.Param{
				Name:      "regression_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Timeout:   3 * time.Minute,
				Val:       pb.Facing_FACING_FRONT,
			},
			testing.Param{
				Name:      "regression_back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Timeout:   3 * time.Minute,
				Val:       pb.Facing_FACING_BACK,
			},
		},
	})
}

func CameraboxAlign(ctx context.Context, s *testing.State) {
	var user, pass string
	facing := s.Param().(pb.Facing)
	manualMode := facing == pb.Facing_FACING_UNSET

	if manualMode {
		user = s.RequiredVar("user")
		pass = s.RequiredVar("pass")
		facingStr := s.RequiredVar("facing")
		facing = pb.Facing(pb.Facing_value["FACING_"+strings.ToUpper(facingStr)])
		if facing == pb.Facing_FACING_UNSET {
			s.Fatal("Unexpected unset facing string value: ", facingStr)
		}
	}

	d := s.DUT()
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to alignment service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Prepare data path on DUT.
	tempdir, err := d.Conn().CommandContext(ctx, "mktemp", "-d", "/tmp/alignment_service_XXXXXX").Output()
	if err != nil {
		s.Fatal("Failed to create remote data path directory: ", err)
	}
	dataPath := strings.TrimSpace(string(tempdir))
	defer d.Conn().CommandContext(ctx, "rm", "-r", dataPath).Output()
	if _, err := linuxssh.PutFiles(
		ctx, d.Conn(), map[string]string{
			s.DataPath("camerabox_align.html"): filepath.Join(dataPath, "camerabox_align.html"),
			s.DataPath("camerabox_align.css"):  filepath.Join(dataPath, "camerabox_align.css"),
			s.DataPath("camerabox_align.js"):   filepath.Join(dataPath, "camerabox_align.js"),
		},
		linuxssh.DereferenceSymlinks); err != nil {
		s.Fatalf("Failed to send data to remote data path %v: %v", dataPath, err)
	}

	ch := make(chan error, 1)
	if manualMode {
		// Start DUT in parallel when display chart on chart tablet for they both require slow login chrome.
		go func() {
			acl := pb.NewAlignmentServiceClient(cl.Conn)
			_, err := acl.ManualAlign(ctx, &pb.ManualAlignRequest{
				DataPath: dataPath,
				Username: user,
				Password: pass,
				Facing:   facing,
			})
			ch <- err
		}()
	}
	var chartAddr string
	if altAddr, ok := s.Var("chart"); ok {
		chartAddr = altAddr
	}
	c, err := chart.New(ctx, s.DUT(), chartAddr, s.DataPath("camerabox_align.svg"), s.OutDir())
	if err != nil {
		s.Fatal("Failed to prepare chart tablet: ", err)
	}
	defer c.Close(ctx, s.OutDir())
	if manualMode {
		if err := <-ch; err != nil {
			s.Fatal("Remote call ManualAlign() failed: ", err)
		}
	} else {
		// For regression mode, the DUT must wait for chart ready first.
		acl := pb.NewAlignmentServiceClient(cl.Conn)
		response, err := acl.CheckAlign(ctx, &pb.CheckAlignRequest{
			DataPath: dataPath,
			Facing:   facing,
		})
		if err != nil {
			s.Fatal("Remote call CheckAlign() failed: ", err)
		}
		if response.Result != pb.TestResult_TEST_RESULT_PASSED {
			s.Error("Align check failed: ", response.Error)
		}
	}

	s.Log("Passed all alignment checks")
}
