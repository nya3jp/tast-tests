// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"

	pp "chromiumos/system_api/patchpanel_proto"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	patchpanel "chromiumos/tast/local/network/patchpanel_client"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const runVhostUserNetTest string = "run-vhost-user-net-test.sh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         VhostUserNet,
		Desc:         "Tests crosvm's vhost-user net device",
		Contacts:     []string{"keiichiw@chromium.org", "crosvm-core@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{vm.ArtifactData(), runVhostUserNetTest},
		SoftwareDeps: []string{
			// TODO(keiichiw): define a new deps for vhost-user-device
			// "vhost_user_devices"
		},
		Pre: vm.Artifact(),
	})
}

type ifreq struct {
	name  [unix.IFNAMSIZ]byte
	flags int16
}

func openTapDevice(device *pp.NetworkDevice) (int, error) {
	const path = "/dev/net/tun"

	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NONBLOCK, 0777)
	if err != nil {
		return -1, errors.Wrapf(err, "failed to open Tap device%s", path)
	}

	ifr := ifreq{}
	copy(ifr.name[:], "VhostUserNet")
	ifr.flags = syscall.IFF_TAP | syscall.IFF_NO_PI | syscall.IFF_VNET_HDR
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		syscall.TUNSETIFF,
		uintptr(unsafe.Pointer(&ifr)),
	)
	if errno != 0 {
		syscall.Close(fd)
		return -1, errors.Errorf("failed to set network interface: %s", errno.Error())
	}

	return fd, nil
}

func VhostUserNet(ctx context.Context, s *testing.State) {
	pc, err := patchpanel.New(ctx)
	if err != nil {
		s.Fatal("Failed to create patchpanel client: ", err)
	}

	cid := 12345
	resp, err := pc.NotifyTerminaVMStartup(ctx, uint32(cid))
	if err != nil {
		s.Fatal("Failed to send NotifyTerminaVMStartup request to patchpanel: ", err)
	}
	defer func() {
		err := pc.NotifyTerminaVMShutdown(ctx, uint32(cid))
		if err != nil {
			s.Error("Failed to send NotifyTerminaVmShutdown request to patchpanel: ", err)
		}
	}()

	device := resp.Device
	subnet := resp.ContainerSubnet
	s.Logf("NotifyTerminaVmStartup: %v %v", device, subnet)

	tapFd, err := openTapDevice(device)
	if err != nil {
		s.Fatal("Failed to open Tap device: ", err)
	}
	defer syscall.Close(tapFd)

	td, err := ioutil.TempDir("/usr/local/tmp", "tast.vm.VhostUserNet.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(td)

	data := s.PreValue().(vm.PreData)

	shared := filepath.Join(td, "shared")
	if err := os.Mkdir(shared, 0755); err != nil {
		s.Fatal("Failed to create shared directory: ", err)
	}

	logFile := filepath.Join(s.OutDir(), "serial.log")

	args := []string{
		"crosvm", "run",
		"-c", "1",
		"-m", "1024",
		"-s", td,
		"--shared-dir", "/:/dev/root:type=fs:cache=always",
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", logFile),
		"--tap-fd", strconv.Itoa(tapFd),
	}

	params := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		"rw",
		fmt.Sprintf("init=%s", s.DataPath(runVhostUserNetTest)),
		"--",
		td,
	}

	args = append(args, "-p", strings.Join(params, " "), data.Kernel)

	output, err := os.Create(filepath.Join(s.OutDir(), "crosvm.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer output.Close()

	s.Log("Running vhost-user net test")
	cmd := testexec.CommandContext(ctx, "prlimit", args...)
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}
}
