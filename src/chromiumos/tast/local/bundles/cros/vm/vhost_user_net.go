// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"encoding/binary"
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
		SoftwareDeps: []string{"vm_host", "vhost_user_devices"},
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
		return 0, errors.Wrapf(err, "failed to open Tap device: %v", device.Ifname)
	}

	if len(device.Ifname) > unix.IFNAMSIZ-1 {
		syscall.Close(fd)
		return 0, errors.Wrapf(err, "too long Ifname: %s", device.Ifname)
	}

	ifr := ifreq{}
	copy(ifr.name[:], device.Ifname)
	ifr.flags = syscall.IFF_TAP | syscall.IFF_NO_PI | syscall.IFF_VNET_HDR
	if _, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		syscall.TUNSETIFF,
		uintptr(unsafe.Pointer(&ifr)),
	); errno != 0 {
		syscall.Close(fd)
		return 0, errors.Errorf("failed to set network interface: %s", errno.Error())
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
		vm.SharedDir("/", "/dev/root", "fs", "always"),
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
		if err := pc.NotifyTerminaVMShutdown(ctx, cid); err != nil {
			testing.ContextLog(ctx, "Failed to notify termina shutdown: ", err)
		}
	}

	fd, err := openTapDevice(resp.Device)
	if err != nil {
		shutdown()
		err = errors.Wrap(err, "failed to open Tap device")
		return
	}

	// Convert BaseAddr into an IP address
	// Note that we need to explicitly change byte order from "network order" (= big endian) to little endian.
	gateway := make(net.IP, 4)
	binary.LittleEndian.PutUint32(gateway[0:], resp.Device.HostIpv4Addr)
	addr := make(net.IP, 4)
	binary.LittleEndian.PutUint32(addr[0:], resp.Device.Ipv4Addr)

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
	td, err := ioutil.TempDir("", "tast.vm.VhostUserNet.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(td)

	// Get Tap Fds
	pc, err := patchpanel.New(ctx)
	if err != nil {
		s.Fatal("Failed to create patchpanel client: ", err)
	}

	// cid will be used by patchpanel to identify VMs.
	// Use values larger than 8192 to guarantee no overlap with crostini/arcvm.
	serverCid := uint32(8193)
	serverTap, serverCleanup, err := getTap(ctx, pc, serverCid)
	if err != nil {
		s.Fatal("Failed to get Tap FD for server: ", err)
	}
	defer serverCleanup()

	clientCid := uint32(8194)
	clientTap, clientCleanup, err := getTap(ctx, pc, clientCid)
	if err != nil {
		s.Fatal("Failed to get Tap FD for client: ", err)
	}
	defer clientCleanup()

	// Start vhost-user-net-device.
	devlog, err := os.Create(filepath.Join(s.OutDir(), "device.log"))
	if err != nil {
		s.Fatal("Failed to create device log file: ", err)
	}
	defer devlog.Close()

	serverSock := filepath.Join(td, "vhost-user-net-server.sock")
	clientSock := filepath.Join(td, "vhost-user-net-client.sock")

	cmdArgs := []string{
		"--tap-fd", fmt.Sprintf("%s,%d", serverSock, serverTap.fd),
		"--tap-fd", fmt.Sprintf("%s,%d", clientSock, clientTap.fd),
	}
	devCmd := testexec.CommandContext(ctx, "vhost-user-net-device", cmdArgs...)
	devCmd.Stdout = devlog
	devCmd.Stderr = devlog
	if err := devCmd.Start(); err != nil {
		s.Fatal("Failed to start vhost-user-net-device: ", err)
	}

	data := s.PreValue().(vm.PreData)
	script := s.DataPath(runVhostUserNetTest)

	// Start server VM
	serverLog := filepath.Join(s.OutDir(), "serial-server.log")
	serverArgs := []string{
		"server",
		serverTap.addr.String(),
		serverTap.gateway.String(),
	}
	serverCmd := getCrosvmCmd(ctx, data.Kernel, serverLog, serverSock, script, serverArgs)
	serverOut, err := os.Create(filepath.Join(s.OutDir(), "crosvm-server.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm server log file: ", err)
	}
	defer serverOut.Close()
	serverCmd.Stdout = serverOut
	serverCmd.Stderr = serverOut

	if err := serverCmd.Start(); err != nil {
		s.Fatal("Failed to run server crosvm: ", err)
	}

	// Start client VM
	clientLog := filepath.Join(s.OutDir(), "serial-client.log")
	clientArgs := []string{
		"client",
		clientTap.addr.String(),
		clientTap.gateway.String(),
		serverTap.addr.String(), // destination address
	}
	clientCmd := getCrosvmCmd(ctx, data.Kernel, clientLog, clientSock, script, clientArgs)
	clientOut, err := os.Create(filepath.Join(s.OutDir(), "crosvm-client.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm client log file: ", err)
	}
	defer clientOut.Close()
	clientCmd.Stdout = clientOut
	clientCmd.Stderr = clientOut

	if err := clientCmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run client crosvm: ", err)
	}

	// Wait for server VM being completed.
	if err := serverCmd.Wait(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to complete server VM: ", err)
	}

	// vhost-user-net device must stop right after all of VMs stopped.
	if err := devCmd.Wait(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to complete vhost-user-net-device: ", err)
	}

	// Check client log
	log, err := ioutil.ReadFile(clientLog)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", clientLog, err)
	}

	if !bytes.Contains(log, []byte("iperf Done.")) {
		s.Fatal("iperf3 didn't run successfully")
	}
}
