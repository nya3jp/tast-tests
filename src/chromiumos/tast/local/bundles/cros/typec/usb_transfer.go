// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"chromiumos/tast/errors"
	deviceSpeed "chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBTransfer,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test USB type-A 2.0/ type-C 3.0 Pendrive detection and RW",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name: "usb2",
			Val:  "480M",
		}, {
			Name: "usb3",
			Val:  "5000M",
		}},
	})
}

func USBTransfer(ctx context.Context, s *testing.State) {
	wantSpeed := s.Param().(string)

	// Verify USB pendrive speed.
	speeds, err := deviceSpeed.MassStorageUSBSpeed(ctx)
	if err != nil {
		s.Fatal("Failed to check for USB speed: ", err)
	}

	speedFound := false
	for _, speed := range speeds {
		if speed == wantSpeed {
			speedFound = true
			break
		}
	}
	if !speedFound {
		s.Fatalf("Unexpected USB device speed: want %q, got %v", wantSpeed, speeds)
	}

	// Source file name.
	transFilename := "file_ogg.ogg"

	sourcePath, err := ioutil.TempDir("", "temp")
	if err != nil {
		s.Fatal("Failed to create temp directory: ", err)
	}
	defer os.RemoveAll(sourcePath)

	// Source file path.
	sourceFilePath := path.Join(sourcePath, transFilename)
	if err := ioutil.WriteFile(sourceFilePath, []byte("test"), 0644); err != nil {
		s.Fatal("Failed to create file in tempdir: ", err)
	}
	defer os.Remove(sourceFilePath)

	localHash, err := calcHash(sourceFilePath)
	if err != nil {
		s.Error("Failed to calculate hash of the source file : ", err)
	}

	dir, err := usbRemovableDirs(ctx)
	if err != nil {
		s.Fatal("Failed to get removable device: ", err)
	}

	// Destination file path.
	destinationFilePath := path.Join(dir[0], transFilename)
	defer os.Remove(destinationFilePath)

	if err := deviceSpeed.CopyFile(sourceFilePath, destinationFilePath); err != nil {
		s.Fatalf("Failed to copy file to %s path", destinationFilePath)
	}

	if err := deviceSpeed.CopyFile(destinationFilePath, sourceFilePath); err != nil {
		s.Fatalf("Failed to copy file to %s path", sourceFilePath)
	}

	destHash, err := calcHash(destinationFilePath)
	if err != nil {
		s.Error("Failed to calculate hash of the destination file : ", err)
	}

	if !bytes.Equal(localHash, destHash) {
		s.Errorf(" The hash doesn't match (destHash path: %q)", destHash)
	}
}

// calcHash checks the checksum for the input file.
func calcHash(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return []byte{}, errors.Wrap(err, "failed to open files")
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return []byte{}, errors.Wrap(err, "failed to calculate the hash of the files")
	}

	return h.Sum(nil), nil
}

// usbRemovableDirs returns the connected removable devices.
func usbRemovableDirs(ctx context.Context) ([]string, error) {
	var mountPaths []string
	info, err := sysutil.MountInfoForPID(sysutil.SelfPID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to mount info")
	}
	for _, i := range info {
		if strings.HasPrefix(i.MountPath, "/media/removable") {
			mountPaths = append(mountPaths, i.MountPath)
		}
	}
	if len(mountPaths) == 0 {
		return nil, errors.New("no mount path found")
	}
	return mountPaths, nil
}
