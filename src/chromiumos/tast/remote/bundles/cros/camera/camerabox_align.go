// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/remote/bundles/cros/camera/pre"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraboxAlign,
		Desc:         "Verifying alignment of chart tablet screen and target facing camera FOV in camerabox setup",
		Data:         []string{pre.AlignChartScene().DataPath(), "camerabox_align.html", "camerabox_align.css", "camerabox_align.js"},
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.camerabox.AlignmentService"},
		Vars:         []string{"chart", "facing", "user", "pass"},
		Pre:          pre.AlignChartScene(),
		Timeout:      20 * time.Minute,
	})
}

func CameraboxAlign(ctx context.Context, s *testing.State) {
	d := s.DUT()
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to alignment service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	user := s.RequiredVar("user")
	pass := s.RequiredVar("pass")
	facingStr := s.RequiredVar("facing")
	facing := pb.Facing(pb.Facing_value["FACING_"+strings.ToUpper(facingStr)])
	if facing == pb.Facing_FACING_UNSET {
		s.Fatal("Unexpected unset facing string value: ", facingStr)
	}

	// Prepare data path on DUT.
	tempdir, err := d.Conn().Command("mktemp", "-d", "/tmp/alignment_service_XXXXXX").Output(ctx)
	if err != nil {
		s.Fatal("Failed to create remote data path directory: ", err)
	}
	dataPath := strings.TrimSpace(string(tempdir))
	defer d.Conn().Command("rm", "-r", dataPath).Output(ctx)
	if _, err := linuxssh.PutFiles(
		ctx, d.Conn(), map[string]string{
			s.DataPath("camerabox_align.html"): filepath.Join(dataPath, "camerabox_align.html"),
			s.DataPath("camerabox_align.css"):  filepath.Join(dataPath, "camerabox_align.css"),
			s.DataPath("camerabox_align.js"):   filepath.Join(dataPath, "camerabox_align.js"),
		},
		linuxssh.DereferenceSymlinks); err != nil {
		s.Fatalf("Failed to send data to remote data path %v: %v", dataPath, err)
	}

	// TODO(b/166370953): Run display chart on tablet and Chrome Remote Desktop on DUT in parallel.
	acl := pb.NewAlignmentServiceClient(cl.Conn)
	if _, err := acl.ManualAlign(ctx, &pb.ManualAlignRequest{
		DataPath: dataPath,
		Username: user,
		Password: pass,
		Facing:   facing,
	}); err != nil {
		s.Fatal("Remote call ManualAlign() failed: ", err)
	}

	s.Log("Passed all alignment checks")
}
