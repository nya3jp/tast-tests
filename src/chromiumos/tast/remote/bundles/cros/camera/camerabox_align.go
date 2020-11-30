// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
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
		Data:         []string{"preview.html", "align.svg"},
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
		s.Fatal("Unexpected unset facing value")
	}
	framePath := filepath.Join(s.OutDir(), "frame.jpg")

	// Prepare temp dir on DUT and copy all testing assets into it.
	tempdir, err := d.Conn().Command("mktemp", "-d", "/tmp/alignment_service_XXXXXX").Output(ctx)
	if err != nil {
		s.Fatal("Failed to create out dir: ", err)
	}
	out := strings.TrimSpace(string(tempdir))
	if _, err := linuxssh.PutFiles(
		ctx, d.Conn(), map[string]string{s.DataPath("preview.html"): filepath.Join(out, "preview.html")},
		linuxssh.DereferenceSymlinks); err != nil {
		s.Fatal("Failed to send preview.html: ", err)
	}

	// Run remote test on DUT.
	acl := pb.NewAlignmentServiceClient(cl.Conn)
	if _, err := acl.Prepare(ctx, &pb.PrepareRequest{OutDir: out, Username: user, Password: pass}); err != nil {
		s.Fatal("Remote call RunTest() failed: ", err)
	}
	defer func() {
		acl.Cleanup(ctx, &empty.Empty{})
	}()

	getPreviewFrame := func(facing pb.Facing, ratio pb.AspectRatio) (image.Image, error) {
		if _, err := acl.GetPreviewFrame(ctx, &pb.GetPreviewFrameRequest{Facing: facing, Ratio: ratio}); err != nil {
			return nil, errors.Wrap(err, "remote call GetPreviewFrame() failed")
		}
		if err := linuxssh.GetFile(ctx, d.Conn(), filepath.Join(out, "frame.jpg"), framePath); err != nil {
			return nil, errors.Wrap(err, "failed to get frame.jpg from DUT")
		}
		f, err := os.Open(framePath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to open frame.jpg")
		}
		img, err := jpeg.Decode(f)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode frame.jpg")
		}
		return img, nil
	}

	checkAligned := func(ratio pb.AspectRatio) bool {
		img, err := getPreviewFrame(facing, ratio)
		if err != nil {
			s.Fatal("Failed to get preview frame: ", err)
		}

		hue := func(c color.Color) float64 {
			intR, intG, intB, _ := c.RGBA()
			r := float64(intR)
			g := float64(intG)
			b := float64(intB)
			max := b
			if r >= g && r >= b {
				max = r
			} else if g >= r && g >= b {
				max = g
			}

			min := b
			if r <= g && r <= b {
				min = r
			} else if g <= r && g <= b {
				min = g
			}

			d := max - min
			if d == 0 {
				// No hue value e.g. white or black
				return -1
			}
			if max == r {
				return math.Mod(float64(g-b)/float64(d), 6) * 60
			}
			if max == g {
				return (float64(b-r)/float64(d) + 2) * 60
			}
			return (float64(r-g)/float64(d) + 4) * 60
		}

		matchColor := func(c color.Color) bool {
			h := hue(c)
			// 80 <= hue <= 140 according to target green pattern
			return 80 <= h && h <= 140
		}

		// Check all boundary pixels fall on target pattern.
		bound := img.Bounds()
		notMatch := 0
		for _, x := range []int{bound.Min.X, bound.Max.X - 1} {
			for y := bound.Min.Y; y < bound.Max.Y; y++ {
				c := img.At(x, y)
				if !matchColor(c) {
					notMatch++
				}
			}
		}
		for _, y := range []int{bound.Min.Y, bound.Max.Y - 1} {
			for x := bound.Min.X; x < bound.Max.X; x++ {
				c := img.At(x, y)
				if !matchColor(c) {
					notMatch++
				}
			}
		}
		if notMatch != 0 {
			return false
		}
		return true
	}

	aspectRatioLog := func(ratio pb.AspectRatio) string {
		if ratio == pb.AspectRatio_AR4X3 {
			return "Aspect ratio 4x3"
		}
		return "Aspect ratio 16x9"
	}

	feedback := func(passed bool, msg string) {
		if _, err := acl.FeedbackAlign(ctx, &pb.FeedbackAlignRequest{Passed: passed, Msg: msg}); err != nil {
			s.Fatalf("Failed to call FeedbackAlign() with %v and massge %v: %v", passed, msg, err)
		}
	}

	manualAligned := func(ratio pb.AspectRatio) {
		cnt := 0
		s.Log("Start manual align mode for ", aspectRatioLog(ratio))
		for {
			aligned := checkAligned(ratio)
			if aligned {
				cnt++
				if cnt >= 5 {
					break
				}
				feedback(true, fmt.Sprintf("Pass check %v align %d times", aspectRatioLog(ratio), cnt))
			} else {
				cnt = 0
				feedback(false, fmt.Sprintf("Check %v align failed", aspectRatioLog(ratio)))
			}

			if err := testing.Sleep(ctx, time.Second); err != nil {
				s.Fatal("Failed to sleep for aligning ", aspectRatioLog(ratio))
			}
		}
		s.Log("Passed manual align mode for ", aspectRatioLog(ratio))
	}

	for {
		manualAligned(pb.AspectRatio_AR4X3)
		manualAligned(pb.AspectRatio_AR16X9)
		s.Log("Wait 5 seconds for fixture settling")
		feedback(true, "Wait 5 seconds for fixture settling")
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to sleep 5 seconds for fixture settling")
		}
		if !checkAligned(pb.AspectRatio_AR4X3) {
			s.Logf("%v is not aligned after fixture settling", aspectRatioLog(pb.AspectRatio_AR4X3))
			continue
		}
		if !checkAligned(pb.AspectRatio_AR16X9) {
			s.Logf("%v is not aligned after fixture settling", aspectRatioLog(pb.AspectRatio_AR16X9))
			continue
		}
		break
	}
	s.Log("Passed all alignment checks")
}
