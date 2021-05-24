// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
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
		SoftwareDeps: []string{"vhost_user_devices"},
		Pre:          vm.Artifact(),
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

func getCrosvmCmd(ctx context.Context, kernel, serialLog, sock, script string, scriptArgs []string) *testexec.Cmd {
	kernParams := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		"rw",
		fmt.Sprintf("init=%s", script),
		"--",
	}
	kernParams = append(kernParams, scriptArgs...)

	ps := vm.NewCrosvmParams(
		kernel,
		vm.SharedDir("/:/dev/root:type=fs:cache=always"),
		vm.VhostUserNet(sock),
		vm.KernelArgs(kernParams...),
		vm.SerialOutput(serialLog),
	)
	args := ps.ToArgs()
	return testexec.CommandContext(ctx, "crosvm", args...)
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
		err := pc.NotifyTerminaVMShutdown(ctx, cid)
		if err != nil {
			testing.ContextLog(ctx, "Failed to notify termina shutdown: ", err)
		}
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
	cid1 := uint32(112)
	tap1, cleanup1, err := getTap(ctx, pc, cid1)
	if err != nil {
		s.Fatal("Failed to get 1st Tap FD: ", err)
	}
	defer cleanup1()

	cid2 := uint32(113)
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
	script := s.DataPath(runVhostUserNetTest)

	// Start VM 1 (server)
	serialLog1 := filepath.Join(s.OutDir(), "serial1.log")
	scriptArgs1 := []string{
		"server",
		tap1.addr.String(),
		tap1.gateway.String(),
	}
	crosvmCmd1 := getCrosvmCmd(ctx, data.Kernel, serialLog1, sock1, script, scriptArgs1)
	output1, err := os.Create(filepath.Join(s.OutDir(), "crosvm1.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer output1.Close()
	crosvmCmd1.Stdout = output1
	crosvmCmd1.Stderr = output1

	s.Log("Start VM 1")
	if err := crosvmCmd1.Start(); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	// Start VM 2 (client)
	serialLog2 := filepath.Join(s.OutDir(), "serial2.log")
	scriptArgs2 := []string{
		"client",
		tap2.addr.String(),
		tap2.gateway.String(),
		tap1.addr.String(), // destination address
	}
	crosvmCmd2 := getCrosvmCmd(ctx, data.Kernel, serialLog2, sock2, script, scriptArgs2)
	output2, err := os.Create(filepath.Join(s.OutDir(), "crosvm2.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer output2.Close()
	crosvmCmd2.Stdout = output2
	crosvmCmd2.Stderr = output2

	s.Log("Start VM 2")
	if err := crosvmCmd2.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	// Wait for VM 1 being completed.
	if err := crosvmCmd1.Wait(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to complete VM 1: ", err)
	}

	// Check client log
	log, err := ioutil.ReadFile(serialLog2)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", serialLog2, err)
	}

	if !bytes.Contains(log, []byte("iperf Done.")) {
		s.Fatal("iperf3 didn't run successfully")
	}
}
