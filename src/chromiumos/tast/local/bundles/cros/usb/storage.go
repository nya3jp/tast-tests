// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usb

import (
	"context"
	"time"
	"path/filepath"
	"io/ioutil"
	"os"

//	"chromiumos/tast/errors"
	"chromiumos/tast/local/usb"
	"chromiumos/tast/local/usb/gadget"
	"chromiumos/tast/local/usb/gadget/storage"
	"chromiumos/tast/local/chrome"
//	"chromiumos/tast/local/vm"
//	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

var (
	imageFiles = []string{"vfat.tgz", "ext4.tgz"}
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Storage,
		Desc:         "USB Storage Device test",
		Contacts:     []string{"tjeznach@chromium.org"},
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(),
//		Pre:          crostini.StartedByDownload(),
		Data:         imageFiles,
		SoftwareDeps: []string{"chrome"},
	})
}

func Storage(ctx context.Context, s *testing.State) {
	// Connect to Crostini instance.
//	pre := s.PreValue().(vm.CrostiniPre)
//	cont := pre.Container

	// prepare image files.
	tmpDir, err := ioutil.TempDir("", "tast.usb.storage")
	if err != nil {
		s.Fatal("Failed to create temporary directory")
	}
	defer os.RemoveAll(tmpDir)

	for _, img := range imageFiles {
		cmd := testexec.CommandContext(ctx, "tar", "-xzf", s.DataPath(img), "-C", tmpDir)
		if err := cmd.Run(); err != nil {
			s.Fatal("Can't prepare image file: ", err)
		}
	}

	// Load kernel modules.
	if err := usb.InstallModules(ctx); err != nil {
		s.Fatal(err)
	}
	defer usb.RemoveModules(ctx)

	g := gadget.NewGadget(usb.DeviceInfo{
		VendorId       : 0x0781,
		ProductId      : 0x5580,
		Manufacturer   : "SanDisk",
		Product        : "SDCZ80 Flash Drive Emulation",
		SerialNumber   : "20100201396000000",
		DeviceRev      : 0x0100,
		UsbRev         : 0x0200,
		DeviceClass    : 0x00,
		DeviceSubClass : 0x00,
		DeviceProtocol : 0x00,
	})
	fv := storage.NewStorage("lun0", filepath.Join(tmpDir, "vfat.img"))
	if err := g.Register(fv); err != nil {
		s.Fatal("register failed: ", err)
	}
	fe := storage.NewStorage("lun1", filepath.Join(tmpDir, "ext4.img"))
	if err := g.Register(fe); err != nil {
		s.Fatal("register failed: ", err)
	}
	if err := g.Start(ctx); err != nil {
		s.Fatal("start failed: ", err)
	}
	if err := g.Bind(usb.LoopbackDevicePort()); err != nil {
		s.Fatal("bind failed: ", err)
	}

	time.Sleep(10 * time.Second)

	g.Unbind()
	g.Stop()

/*
	dev, err := usbg.NewDevice(ctx, desc)
	if err != nil {
		s.Fatal(errors.Wrap(err, "can not create emulated USB device"))
	}
	defer dev.Close()

	go serial.Run(ctx, dev)

	s.Log("Connecting emulated USB device")
	dev.Plug("dummy_udc.0")

	time.Sleep(10 * time.Second)

	// check USB on host system.
	cmd := testexec.CommandContext(ctx, "lsusb", "-d", "10c4:ea60")
	if err := cmd.Run(); err != nil {
		s.Fatal("Can't find USB device on host: ", err)
	}
	cmd = testexec.CommandContext(ctx, "ls", "-l", "/dev/ttyUSB0")
	if err := cmd.Run(); err != nil {
		s.Fatal("Can't find USB device on host: ", err)
	}


	// Attach USB to VM.
	usb_path := dev.GetPath()
	port, err := cont.VM.AttachUsbDevice(ctx, usb_path)
	if err != nil {
		s.Log("Failed to attach USB device: ", err)
		time.Sleep(90 * time.Second)
		s.Fatal("terminating")
	}
	s.Logf("Device '%v' attached at port %d\n", usb_path, port)

	// wait for guest kernel to process hot-plug event.
	time.Sleep(5 * time.Second)

	// verify device visibility in guest system.
	cmd = cont.Command(ctx, "lsusb", "-v", "-d", "10c4:ea60")
	if err := cmd.Run(); err != nil {
		s.Fatal("Can't find USB device on VM: ", err)
	} else {
		cmd.DumpLog(ctx)
	}

	cmd = cont.Command(ctx, "ls", "-l", "/dev/ttyUSB0")
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to run 'ls -l /dev/ttyUSB0': ", err)
	} else {
		cmd.DumpLog(ctx)
	}

	// check USB on host system.
	cmd = testexec.CommandContext(ctx, "lsusb", "-v", "-d", "10c4:ea60")
	if err := cmd.Run(); err != nil {
		s.Log("Can't find USB device on host: ", err)
	} else {
		cmd.DumpLog(ctx)
	}
	cmd = testexec.CommandContext(ctx, "ls", "-l", "/dev/ttyUSB0")
	if err := cmd.Run(); err != nil {
		s.Log("Can't find USB device on host: ", err)
	} else {
		cmd.DumpLog(ctx)
	}

	// optional detach. unexpected removal will not produce DetachUsbDevice event.
	if !unexpected_disconnect {
		if err := cont.VM.DetachUsbDevice(ctx, port); err != nil {
			s.Fatal("failed to diconnect: ", err)
		}
	}

	dev.Unplug()

	s.Logf("Device '%v' unplugged from port %d\n", usb_path, port)
	time.Sleep(3 * time.Second)

	// Check USB device at host and guest systems
	cmd = testexec.CommandContext(ctx, "lsusb", "-d", "10c4:ea60")
	if err := cmd.Run(); err != nil {
		s.Log("Can't find USB device on host: ", err)
	}

	cmd = cont.Command(ctx, "lsusb", "-d", "10c4:ea60")
	if err := cmd.Run(); err != nil {
		s.Log("Can't find USB device on VM: ", err)
	} else {
		cmd.DumpLog(ctx)
	}
*/
}
