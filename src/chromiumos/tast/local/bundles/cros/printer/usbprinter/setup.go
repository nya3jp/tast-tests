// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package usbprinter provides an interface to configure and attach a virtual
// USB printer onto the system to be used for testing.
package usbprinter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"

	"chromiumos/tast/diff"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// idInPDFRegex matches the "ID" embedded in the PDF file which uniquely
// identifies the document. This line is removed so that file comparison will
// pass.
var idInPDFRegex = regexp.MustCompile("(?m)^.*\\/ID \\[<[a-f0-9]+><[a-f0-9]+>\\] >>[\r\n]")

// TODO(crbug.com/973637): Investigate why it is that CUPS is inconsistent on
// settings the values for the "For" and "Title" fields in the resulting PDF.
// Once the root cause is determined and fixed we should perform the PDF
// comparison without stripping the fields and can remove these regexes.

// usernameInPDFRegex matches the line with "For" field embedded in the PDF.
var usernameInPDFRegex = regexp.MustCompile("(?m)^%%For: \\(\\w+\\)$")

// documentTitleInPDFRegex matches the line with the "Title" field embedded in
// the PDF.
var documentTitleInPDFRegex = regexp.MustCompile("(?m)^%%Title: \\([\\w\\.]+\\)$")

// DevInfo contains information used to identify a USB device.
type DevInfo struct {
	// VID contains the device's vendor ID.
	VID string
	// PID contains the devices's product ID.
	PID string
}

// LoadPrinterIDs loads the JSON file located at path and attempts to extract
// the "vid" and "pid" from the USB device descriptor which should be defined
// in path.
func LoadPrinterIDs(path string) (devInfo DevInfo, err error) {
	f, err := os.Open(path)
	if err != nil {
		return devInfo, errors.Wrapf(err, "failed to open %s", path)
	}
	defer f.Close()

	var cfg struct {
		DevDesc struct {
			Vendor  int `json:"idVendor"`
			Product int `json:"idProduct"`
		} `json:"device_descriptor"`
	}

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return devInfo, errors.Wrapf(err, "failed to decode JSON in %s", path)
	}

	return DevInfo{fmt.Sprintf("%04x", cfg.DevDesc.Vendor), fmt.Sprintf("%04x", cfg.DevDesc.Product)}, nil
}

// LoadPrinterName loads the JSON file located at path and attempts to extract
// the "string_descriptors" from the USB device descriptor which should be defined
// in path.
func LoadPrinterName(path string) (printerName string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return printerName, errors.Wrapf(err, "failed to open %s", path)
	}
	defer f.Close()

	var name struct {
		parts []string `json:"string_descriptors"`
	}

	if err := json.NewDecoder(f).Decode(&name); err != nil {
		return printerName, errors.Wrapf(err, "failed to decode JSON in %s", path)
	}

	return fmt.Sprintf("%q %q (USB)", name.parts[0], name.parts[1]), nil
}

// InstallModules installs the "usbip_core" and "vhci-hcd" kernel modules which
// are required by usbip in order to bind the virtual printer to the system.
func InstallModules(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "modprobe", "-a", "usbip_core",
		"vhci-hcd")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to install usbip kernel modules")
	}
	return nil
}

// RemoveModules removes the "usbip_core" and "vhci-hcd" kernel modules that
// were installed during the test run.
func RemoveModules(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "modprobe", "-r", "-a", "vhci-hcd",
		"usbip_core")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to remove usbip kernel modules")
	}
	return nil
}

// Start sets up and runs a new virtual printer and attaches it to the system
// using USBIP. The given descriptors and attributes provide the virtual printer
// with paths to the USB descriptors and IPP attributes files respectively. The
// path to the file to write received documents is specified by record. The
// returned command is already started and must be stopped (by calling its Kill
// and Wait methods) when testing is complete.
func Start(ctx context.Context, devInfo DevInfo, descriptors, attributes, record string) (cmd *testexec.Cmd, err error) {
	testing.ContextLog(ctx, "Starting virtual printer")
	launch := testexec.CommandContext(ctx, "stdbuf", "-o0", "virtual-usb-printer", "--descriptors_path="+descriptors, "--attributes_path="+attributes, "--record_doc_path="+record)

	p, err := launch.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := launch.Start(); err != nil {
		return nil, errors.Wrapf(err, "failed to start %v", launch.Args)
	}
	cmdToKill := launch
	defer func() {
		if cmdToKill != nil {
			launch.Kill()
			launch.Wait()
		}
	}()

	// Ensure that virtual-usb-printer has launched successfully.
	if err := waitLaunch(p); err != nil {
		return nil, errors.Wrap(err, "failed to launch virtual printer")
	}
	testing.ContextLog(ctx, "Started virtual printer")

	// Need to read from the pipe so that the virtual printer doesn't block on
	// writing to stdout
	go io.Copy(ioutil.Discard, p)

	// Begin waiting for udev event.
	udevCh := make(chan error, 1)
	go func() {
		udevCh <- waitEvent(ctx, "add", devInfo)
	}()

	// Attach the virtual printer to the system using the "usbip attach" command.
	testing.ContextLog(ctx, "Attaching virtual printer")
	attach := testexec.CommandContext(ctx, "usbip", "attach", "-r", "localhost",
		"-b", "1-1")
	if err := attach.Run(); err != nil {
		return nil, errors.Wrap(err, "failed to attach virtual usb printer")
	}

	// Wait for a signal from udevadm to see if the device was successfully
	// attached.
	testing.ContextLog(ctx, "Waiting for udev event")
	select {
	case err := <-udevCh:
		if err != nil {
			return nil, err
		}
		testing.ContextLog(ctx, "Found add event")
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "didn't get udev event")
	}

	// Run lsusb to sanity check that that the device is actually connected.
	id := fmt.Sprintf("%s:%s", devInfo.VID, devInfo.PID)
	checkAttached := testexec.CommandContext(ctx, "lsusb", "-d", id)
	if err := checkAttached.Run(); err != nil {
		checkAttached.DumpLog(ctx)
		return nil, errors.Wrap(err, "printer was not successfully attached")
	}

	cmdToKill = nil
	return launch, nil
}

func cleanPDFContents(f string) (string, error) {
	bytes, err := ioutil.ReadFile(f)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read file %s", f)
	}

	content := string(bytes)
	for _, r := range []*regexp.Regexp{idInPDFRegex, usernameInPDFRegex, documentTitleInPDFRegex} {
		content = r.ReplaceAllLiteralString(content, "")
	}
	return content, nil
}

// compareFiles performs a diff between the given files output and golden. If
// the contents of the files are not the same then the result from the diff
// command will be written to diffPath.
func compareFiles(ctx context.Context, output, golden, diffPath string) error {
	result, err := cleanPDFContents(output)
	if err != nil {
		return err
	}

	expected, err := cleanPDFContents(golden)
	if err != nil {
		return err
	}

	testing.ContextLogf(ctx, "Comparing files %v and %v", output, golden)
	diff, err := diff.Diff(result, expected)
	if err != nil {
		return errors.Wrap(err, "unexpected diff output")
	}
	if diff != "" {
		testing.ContextLog(ctx, "Dumping diff to ", diffPath)
		if err := ioutil.WriteFile(diffPath, []byte(diff), 0644); err != nil {
			testing.ContextLog(ctx, "Failed to dump diff: ", err)
		}
		return errors.New("result file did not match the expected file")
	}
	return nil
}
