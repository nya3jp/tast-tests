// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/camera/chart"
	"chromiumos/tast/common/hps/hpsutil"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/crosserverutil"
	uipb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

const (
	picFile       = "IMG_7451.jpg"
	onePersonFile = "person-present.html"
	noPersonFile  = "no-person-present.html"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraboxAutodim,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the HPS can dim the screen when human presence is detected",
		Data: []string{hpsutil.PersonPresentPageArchiveFilename,
			hpsutil.P2PowerCycleFilename},
		Contacts: []string{
			"eunicesun@google.com",
			"mblsha@google.com",
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:camerabox", "group:hps", "hps_perbuild"},
		Timeout:      6 * time.Minute,
		SoftwareDeps: []string{"hps", "chrome", caps.BuiltinCamera},
		Vars:         []string{"tablet", "grpcServerPort"},
	})
}

func CameraboxAutodim(ctx context.Context, s *testing.State) {
	grpcServerPort := crosserverutil.DefaultGRPCServerPort
	if portStr, ok := s.Var("grpcServerPort"); ok {
		if portInt, err := strconv.Atoi(portStr); err == nil {
			grpcServerPort = portInt
		}
	}

	// Shorten context to allow for cleanup.
	ctxShort, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()
	d := s.DUT()
	cl, err := crosserverutil.Dial(ctxShort, s.DUT(), "localhost", grpcServerPort, true)

	if err != nil {
		s.Fatal("Failed to connect to the DUT: ", err)
	}
	defer cl.Close(ctx)

	// TODO: move the following to a common util packages when there are more tests.
	// This will be addressed in this bug b/225989226
	powercycleTmpDir, err := d.Conn().CommandContext(ctx, "mktemp", "-d", "/tmp/powercycle_XXXXX").Output()
	if err != nil {
		s.Fatal("Failed to create test directory under /tmp for putting powercycle file: ", err)
	}
	powercycleDirPath := strings.TrimSpace(string(powercycleTmpDir))
	powercycleFilePath := filepath.Join(powercycleDirPath, hpsutil.P2PowerCycleFilename)
	defer d.Conn().CommandContext(ctx, "rm", "-r", powercycleDirPath).Output()
	if _, err := linuxssh.PutFiles(
		ctx, d.Conn(),
		map[string]string{
			s.DataPath(hpsutil.P2PowerCycleFilename): powercycleFilePath,
		},
		linuxssh.DereferenceSymlinks,
	); err != nil {
		s.Fatalf("Failed to send data to remote data path %v: %v", powercycleFilePath, err)
	}
	testing.ContextLog(ctx, "Sending file to dut, path being: ", powercycleFilePath)

	// Extract files from tar.
	archive := s.DataPath(hpsutil.PersonPresentPageArchiveFilename)
	dir, err := testexec.CommandContext(ctx, "dirname", archive).Output()
	if err != nil {
		s.Fatal("Failed to get dirname: ", err)
	}
	dirPath := strings.TrimSpace(string(dir))
	testing.ContextLog(ctx, "Dirpath: ", dirPath)

	tarOut, err := testexec.CommandContext(ctx, "tar", "--strip-components=1", "-xvf", archive, "-C", dirPath).Output()
	testing.ContextLog(ctx, "Extracting following files: ", string(tarOut))
	if err != nil {
		s.Fatal("Failed to untar test artifacts: ", err)
	}
	// Creating hps context.
	hctx, err := hpsutil.NewHpsContext(ctx, powercycleFilePath, hpsutil.DeviceTypeBuiltin, s.OutDir(), d.Conn())
	if err != nil {
		s.Fatal("Error creating HpsContext: ", err)
	}

	// Connecting to the other tablet that will render the picture.
	var chartAddr string
	if altAddr, ok := s.Var("tablet"); ok {
		chartAddr = altAddr
	}

	picture := filepath.Join(dirPath, picFile)
	chartPaths := []string{
		filepath.Join(dirPath, onePersonFile),
		filepath.Join(dirPath, noPersonFile)}
	filePaths := append(chartPaths, picture)

	c, hostPaths, err := chart.New(ctx, d, chartAddr, s.OutDir(), filePaths)

	cs := uipb.NewChromeServiceClient(cl.Conn)
	loginReq := &uipb.NewRequest{}
	if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cs.Close(ctx, &empty.Empty{})

	c.Display(ctx, hostPaths[0])

	// Get brightness for the first time here.
	brightness, err := getBrightness(ctx, d.Conn())
	if err != nil {
		s.Fatal("Error failed to get brightness: ", err)
	}
	testing.ContextLog(ctx, "Brightness: ", brightness)

	c.Display(ctx, hostPaths[1])

	if err := testing.Poll(hctx.Ctx, func(ctx context.Context) error {
		autodimBrightness, err := getBrightness(ctx, d.Conn())
		if err != nil {
			return err
		}
		if autodimBrightness >= brightness {
			return errors.Errorf("Auto dim failed. Before human presence: %f, After human presence: %f", brightness, autodimBrightness)
		}
		if autodimBrightness == 0 {
			return errors.New("Screen is completely dark")
		}

		if autodimBrightness < brightness && autodimBrightness != 0 {
			return nil
		}
		return errors.New("Brightness not changed")
	}, &testing.PollOptions{
		// As this one is not testing the quick dim,
		// The default dimming time for a test user is 2mins.
		Interval: 100 * time.Millisecond,
		Timeout:  130 * time.Second,
	}); err != nil {
		s.Fatal("Unexpected brightness change: ", err)
	}
}

func getBrightness(ctx context.Context, conn *ssh.Conn) (float64, error) {
	output, err := conn.CommandContext(ctx, "dbus-send", "--system",
		"--print-reply", "--type=method_call", "--dest=org.chromium.PowerManager", "/org/chromium/PowerManager",
		"org.chromium.PowerManager.GetScreenBrightnessPercent").Output()
	if err != nil {
		testing.ContextLog(ctx, "Getting brightness failed")
		return -1, err
	}

	mregex := regexp.MustCompile(`(.+)double ([0-9]+)`)
	result := mregex.FindStringSubmatch(strings.ToLower(string(output)))
	if len(result) < 2 {
		return -1, errors.New("no brightness found")
	}

	value, err := strconv.ParseFloat(result[2], 64)
	if err != nil {
		return -1, errors.Wrapf(err, "Conversion failed: %q", result[1])
	}
	return value, nil
}
