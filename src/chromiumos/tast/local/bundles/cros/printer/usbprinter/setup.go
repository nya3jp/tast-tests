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
	"strings"

	"chromiumos/tast/diff"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// LoadPrinterIDs loads the JSON file located at path and attempts to extract
// the "vid" and "pid" from the USB device descriptor which should be defined
// in path.
func LoadPrinterIDs(path string) (string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to open %s", path)
	}
	defer f.Close()

	var cfg struct {
		DevDesc struct {
			Vendor  int `json:"idVendor"`
			Product int `json:"idProduct"`
		} `json:"device_descriptor"`
	}

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return "", "", errors.Wrapf(err, "failed to decode JSON in %s", path)
	}
	return fmt.Sprintf("%04x", cfg.DevDesc.Vendor),
		fmt.Sprintf("%04x", cfg.DevDesc.Product), nil
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

// clearPipe continuously reads data from the pipe which contains output from
// the virtual printer. This function is used in order to prevent the virtual
// printer from being blocked on performing output.
func clearPipe(r io.Reader) {
	for {
		b := make([]byte, 8)
		r.Read(b)
	}
}

// Start sets up and runs a new virtual printer and attaches it to the system
// using USBIP. The returned command is already started and must be stopped (by
// calling its Kill and Wait methods) when testing is complete.
func Start(ctx context.Context, vid, pid, descriptors, attributes, record string) (cmd *testexec.Cmd, err error) {
	testing.ContextLog(ctx, "Starting virtual printer")
	descriptorArg := "--descriptors_path=" + descriptors
	attributesArg := "--attributes_path=" + attributes
	recordArg := "--record_doc_path=" + record

	launch := testexec.CommandContext(ctx, "stdbuf", "-o0", "virtual-usb-printer",
		descriptorArg, attributesArg, recordArg)

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
	go clearPipe(p)

	// Begin waiting for udev event.
	udevCh := make(chan error, 1)
	go func() {
		udevCh <- waitEvent(ctx, "add", vid, pid)
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
	id := fmt.Sprintf("%s:%s", vid, pid)
	checkAttached := testexec.CommandContext(ctx, "lsusb", "-d", id)
	if err := checkAttached.Run(); err != nil {
		checkAttached.DumpLog(ctx)
		return nil, errors.Wrap(err, "printer was not successfully attached")
	}

	cmdToKill = nil
	return launch, nil
}

// cupsAddPrinter adds a new virtual USB printer using CUPS. If attributes is
// non-empty then the printer will be configured using IPPUSB. Otherwise, it is
// configured as a generic USB printing using the given ppd.
func cupsAddPrinter(ctx context.Context, vid, pid, attributes, ppd string) error {
	var uri string
	var lpadmin *testexec.Cmd
	if attributes != "" {
		uri = fmt.Sprintf("ippusb://%s_%s/ipp/print", vid, pid)
		lpadmin = testexec.CommandContext(ctx, "lpadmin", "-p", "virtual-test",
			"-v", uri, "-m", "everywhere", "-E")
	} else {
		uri = fmt.Sprintf("usb://%s/%s", vid, pid)
		lpadmin = testexec.CommandContext(ctx, "lpadmin", "-p", "virtual-test",
			"-v", uri, "-P", ppd, "-E")
	}

	testing.ContextLog(ctx, "Adding printer to CUPS")
	err := lpadmin.Run()
	return err
}

// cupsRemovePrinter removes the printer that was configured for testing.
func cupsRemovePrinter(ctx context.Context) error {
	lpadmin := testexec.CommandContext(ctx, "lpadmin", "-x", "virtual-test")
	err := lpadmin.Run()
	return err
}

// cupsStartPrintJob starts a new print job for the file toPrint. Returns the ID
// of the newly created job if successful.
func cupsStartPrintJob(ctx context.Context, toPrint string) (string, error) {
	lp := testexec.CommandContext(ctx, "lp", "-d", "virtual-test", "--", toPrint)
	testing.ContextLog(ctx, "Starting print job")
	output, err := lp.Output()
	if err != nil {
		return "", err
	}

	r, err := regexp.Compile("virtual-test-[0-9]+")
	if err != nil {
		return "", err
	}

	if job := r.FindString(string(output)); job != "" {
		testing.ContextLog(ctx, "Started job ", job)
		return job, nil
	}
	return "", errors.New("failed to find prompt for print job started")
}

// jobCompleted checks whether or not the given print job has been marked as
// completed.
func jobCompleted(ctx context.Context, job string) (bool, error) {
	lpstat := testexec.CommandContext(ctx, "lpstat", "-W", "completed", "-o",
		"virtual-test")

	output, err := lpstat.Output()
	if err != nil {
		return false, err
	}

	if strings.Contains(string(output), job) {
		testing.ContextLog(ctx, "Found job: ", job)
		return true, nil
	}

	return false, nil
}

// waitCompleted continuously calls jobCompleted to see if the given job ID has
// been marked as completed.
func waitCompleted(ctx context.Context, job string) error {
	testing.ContextLog(ctx, "Waiting for ", job, " to complete")
	err := testing.Poll(ctx, func(ctx context.Context) error {
		done, err := jobCompleted(ctx, job)
		if err != nil {
			return err
		}
		if done {
			testing.ContextLog(ctx, "Job ", job, " is completed")
			return nil
		}
		return errors.New("job " + job + " is not done yet")
	}, &testing.PollOptions{})
	return err
}

// removeFile removes file from the system.
func removeFile(ctx context.Context, file string) error {
	testing.ContextLog(ctx, "Removing file ", file)
	rm := testexec.CommandContext(ctx, "rm", file)
	if err := rm.Run(); err != nil {
		return errors.Wrapf(err, "failed to remove file %s", file)
	}
	return nil
}

// cleanFile loads the contents of the given file f and removes lines which
// would cause a file comparison to fail. Returns a string which contains the
// cleaned file contents.
func cleanFile(f string) (string, error) {
	bytes, err := ioutil.ReadFile(f)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read file %s", f)
	}

	// Matches the "ID" embedded in the PDF file which uniquely identifies the
	// document. This line is removed so that file comparison will pass.
	r, err := regexp.Compile("(?m)^.*\\/ID \\[<[a-f0-9]+><[a-f0-9]+>\\] >>[\r\n]")
	if err != nil {
		return "", errors.Wrap(err, "failed to compile regex")
	}
	return r.ReplaceAllLiteralString(string(bytes), ""), nil
}

// compareFiles performs a diff between the given files output and golden. If
// the contents of the files are not the same then the result from the diff
// command will be written to diffPath.
func compareFiles(ctx context.Context, output, golden, diffPath string) error {
	result, err := cleanFile(output)
	if err != nil {
		return err
	}

	expected, err := cleanFile(golden)
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Comparing files ", output, " and ", golden)
	diff, err := diff.Diff(result, expected)
	if err != nil {
		return errors.Wrap(err, "unexpected diff output")
	}
	if diff != "" {
		testing.ContextLog(ctx, "Dumping diff to ", diffPath)
		if err := ioutil.WriteFile(diffPath, []byte(diff), 0644); err != nil {
			return errors.Wrap(err, "failed to dump diff")
		}
		return errors.New("result file did not match the expected file")
	}
	return nil
}
