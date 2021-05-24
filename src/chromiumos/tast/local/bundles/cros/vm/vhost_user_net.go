// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
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
			// TODO(keiichiw): define a new deps for vhost-user-device before submitting this CL.
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

	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		return -1, errors.Wrapf(err, "failed to open Tap device: %s", path)
	}

	if len(device.Ifname) > unix.IFNAMSIZ-1 {
		syscall.Close(fd)
		return -1, errors.Wrapf(err, "too long Ifname: %s", device.Ifname)
	}

	ifr := ifreq{}
	copy(ifr.name[:], device.Ifname)
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

func getCrosvmCmd(ctx context.Context, vmID int, init, td, kernel, outDir, sock string, sArgs []string) (*testexec.Cmd, error) {
	// TODO(keiichiw): Can I reuse vm.NewCrosvm?
	logFile := filepath.Join(outDir, fmt.Sprintf("serial%d.log", vmID))

	args := []string{
		"run",
		"-c", "1",
		"-m", "1024",
		"-s", td,
		"--shared-dir", "/:/dev/root:type=fs:cache=always",
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", logFile),
		"--vhost-user-net", sock,
	}

	params := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		"rw",
	}

	params = append(params, fmt.Sprintf("init=%s", init), "--")
	params = append(params, sArgs...)

	args = append(args, "-p", strings.Join(params, " "), kernel)

	testing.ContextLog(ctx, "command: ", args)

	cmd := testexec.CommandContext(ctx, "crosvm", args...)

	return cmd, nil
}

type tapDevice struct {
	fd      int
	addr    net.IP
	gateway net.IP
}

func getTap(ctx context.Context, pc *patchpanel.Client, cid uint32) (device tapDevice, cleanup func(), err error) {
	resp, err := pc.NotifyTerminaVMStartup(ctx, cid)
	if err != nil {
		err = errors.Wrap(err, "failed to send NotifyTerminaVMStartup request to patchpanel")
		return
	}
	shutdown := func() {
		pc.NotifyTerminaVMShutdown(ctx, cid)
		// TODO(keiichiw): we can't do anything for error here?
	}

	testing.ContextLogf(ctx, "Device=%+v", resp.Device)
	fd, err := openTapDevice(resp.Device)
	if err != nil {
		shutdown()
		err = errors.Wrap(err, "failed to open Tap device")
		return
	}

	// Convert BaseAddr into an IP address
	// Note that it's serialized in "network order". i.e. big endian.
	base := resp.Device.Ipv4Subnet.BaseAddr
	a, b, c, d := byte(base&0xff), byte((base>>8)&0xff), byte((base>>16)&0xff), byte((base>>24)&0xff)

	gateway := net.IPv4(a, b, c, d+1)
	addr := net.IPv4(a, b, c, d+2)

	device = tapDevice{
		fd:      fd,
		addr:    addr,
		gateway: gateway,
	}

	cleanup = func() {
		shutdown()
		syscall.Close(fd)
	}

	return
}

func VhostUserNet(ctx context.Context, s *testing.State) {
	td, err := ioutil.TempDir("/usr/local/tmp", "tast.vm.VhostUserNet.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(td)

	// Get Tap Fds
	pc, err := patchpanel.New(ctx)
	if err != nil {
		s.Fatal("Failed to create patchpanel client: ", err)
	}

	// Use a big number as cid not to use the same value as an existing VM.
	cid1 := uint32(101)
	tap1, cleanup1, err := getTap(ctx, pc, cid1)
	if err != nil {
		s.Fatal("Failed to get 1st Tap FD: ", err)
	}
	defer cleanup1()

	cid2 := uint32(102)
	tap2, cleanup2, err := getTap(ctx, pc, cid2)
	if err != nil {
		s.Fatal("Failed to get 2nd Tap FD: ", err)
	}
	defer cleanup2()

	// Start vhost-user-net-device.
	devlog, err := os.Create(filepath.Join(s.OutDir(), "device.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer devlog.Close()

	sock1 := filepath.Join(td, "vhost-user-net1.sock")
	sock2 := filepath.Join(td, "vhost-user-net2.sock")

	cmdArgs := []string{
		"--tap-fd", fmt.Sprintf("%s,%d", sock1, tap1.fd),
		"--tap-fd", fmt.Sprintf("%s,%d", sock2, tap2.fd),
	}
	s.Log("Running vhost-user net device")
	devCmd := testexec.CommandContext(ctx, "vhost-user-net-device", cmdArgs...)
	devCmd.Stdout = devlog
	devCmd.Stderr = devlog
	go func() {
		if err := devCmd.Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to run vhost-user-net-device: ", err)
		}
	}()

	data := s.PreValue().(vm.PreData)

	// VM 1 (server)
	sArgs1 := []string{
		"server",
		tap1.addr.String(), tap1.gateway.String(),
	}
	crosvmCmd1, err := getCrosvmCmd(ctx, 1, s.DataPath(runVhostUserNetTest), td, data.Kernel, s.OutDir(), sock1, sArgs1)
	if err != nil {
		s.Fatal("Failed to create crosvm command: ", err)
	}

	output, err := os.Create(filepath.Join(s.OutDir(), "crosvm.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer output.Close()
	crosvmCmd1.Stdout = output
	crosvmCmd1.Stderr = output

	go func() {
		s.Log("Running crosvm run")
		if err := crosvmCmd1.Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to run crosvm: ", err)
		}
	}()

	// VM 2 (client)
	sArgs2 := []string{
		"client",
		tap2.addr.String(), tap2.gateway.String(),
		tap1.addr.String(), // destination address
	}
	crosvmCmd2, err := getCrosvmCmd(ctx, 2, s.DataPath(runVhostUserNetTest), td, data.Kernel, s.OutDir(), sock2, sArgs2)
	if err != nil {
		s.Fatal("Failed to create crosvm command: ", err)
	}
	output2, err := os.Create(filepath.Join(s.OutDir(), "crosvm2.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer output2.Close()
	crosvmCmd2.Stdout = output2
	crosvmCmd2.Stderr = output2

	s.Log("Running crosvm run")
	if err := crosvmCmd2.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}
}
