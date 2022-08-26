// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bios

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	commonbios "chromiumos/tast/common/firmware/bios"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ServoHostCommandRunner runs a command on the servo host. Normally this is the servo proxy.
type ServoHostCommandRunner interface {
	// RunCommand execs a command as the root user.
	RunCommand(ctx context.Context, asRoot bool, name string, args ...string) error
	// OutputCommand execs a command as the root user and returns stdout
	OutputCommand(ctx context.Context, asRoot bool, name string, args ...string) ([]byte, error)
	// GetFile copies a remote file to a local file
	GetFile(ctx context.Context, asRoot bool, remoteFile, localFile string) error
	// PutFiles copies a local files to servo host files
	PutFiles(ctx context.Context, asRoot bool, fileMap map[string]string) error
}

// NewRemoteImage creates an Image object representing the currently loaded BIOS image. If you pass in a section, only that section will be read.
func NewRemoteImage(ctx context.Context, runner ServoHostCommandRunner, programmer string, section commonbios.ImageSection, extraFlashromArgs []string) (*commonbios.Image, error) {
	localTempFile, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, errors.Wrap(err, "creating tmpfile for image contents")
	}
	localTempFileName := localTempFile.Name()
	defer os.Remove(localTempFileName)
	if err = localTempFile.Close(); err != nil {
		return nil, errors.Wrap(err, "closing local temp file")
	}

	remoteTmpFile, err := runner.OutputCommand(ctx, true, "mktemp", "-u", "-p", "/var/tmp", "-t", "fwimgXXXXXX")
	if err != nil {
		return nil, errors.Wrap(err, "creating remote temp file")
	}
	remoteTempFileName := strings.TrimSuffix(string(remoteTmpFile), "\n")
	defer runner.RunCommand(ctx, true, "rm", "-f", remoteTempFileName)

	testing.ContextLog(ctx, "Running flashrom (on servohost)")
	frArgs := []string{"-p", programmer, "-r"}
	isOneSection := section != ""
	if isOneSection {
		frArgs = append(frArgs, "-i", fmt.Sprintf("%s:%s", section, remoteTempFileName))
	} else {
		frArgs = append(frArgs, remoteTempFileName)
	}
	frArgs = append(frArgs, extraFlashromArgs...)
	if err := runner.RunCommand(ctx, true, "flashrom", frArgs...); err != nil {
		// TODO(b/185271203): Some debugging that should be removed later
		output, err := runner.OutputCommand(ctx, true, "ls", "-l", remoteTempFileName)
		testing.ContextLogf(ctx, "ls -l %s: %s", remoteTempFileName, string(output))
		if err != nil {
			testing.ContextLogf(ctx, "ls failed: %s", err)
		}
		output, err = runner.OutputCommand(ctx, true, "df", "-h", remoteTempFileName)
		testing.ContextLogf(ctx, "df -h %s: %s", remoteTempFileName, string(output))
		if err != nil {
			testing.ContextLogf(ctx, "df failed: %s", err)
		}
		return nil, errors.Wrap(err, "could not read firmware host image")
	}
	testing.ContextLog(ctx, "Copying image from servohost to localhost")
	if err := runner.GetFile(ctx, true, remoteTempFileName, localTempFileName); err != nil {
		return nil, errors.Wrapf(err, "copy remote %s to local %s", remoteTempFileName, localTempFileName)
	}

	data, err := ioutil.ReadFile(localTempFileName)
	if err != nil {
		return nil, errors.Wrap(err, "could not read firmware host image contents")
	}
	var info map[commonbios.ImageSection]commonbios.SectionInfo
	if !isOneSection {
		testing.ContextLog(ctx, "Running dump_fmap")
		fmap, err := testexec.CommandContext(ctx, "dump_fmap", "-p", localTempFileName).Output(testexec.DumpLogOnError)
		if err != nil {
			return nil, errors.Wrap(err, "could not dump_fmap on firmware host image")
		}
		info, err = commonbios.ParseSections(string(fmap))
		if err != nil {
			return nil, errors.Wrap(err, "could not parse dump_fmap output")
		}
	} else {
		info = make(map[commonbios.ImageSection]commonbios.SectionInfo)
		info[section] = commonbios.SectionInfo{
			Start:  0,
			Length: uint(len(data)),
		}
	}
	return commonbios.NewImageFromData(data, info), nil
}

// WriteRemoteFlashrom writes the current data in the specified section into flashrom.
func WriteRemoteFlashrom(ctx context.Context, runner ServoHostCommandRunner, programmer string, i *commonbios.Image, sec commonbios.ImageSection, extraFlashromArgs []string) error {
	dataRange, ok := i.Sections[sec]
	if !ok {
		return errors.Errorf("section %q is not recognized", string(sec))
	}

	localTempFile, err := ioutil.TempFile("", "")
	if err != nil {
		return errors.Wrap(err, "creating tmpfile for image contents")
	}
	localImageFileName := localTempFile.Name()
	defer os.Remove(localImageFileName)
	if err = localTempFile.Close(); err != nil {
		return errors.Wrap(err, "closing local temp file")
	}

	dataToWrite := i.Data[dataRange.Start : dataRange.Start+dataRange.Length]

	f, err := os.OpenFile(localImageFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	_, err = f.Write(dataToWrite)
	if err1 := f.Close(); err == nil {
		err = err1
	}

	remoteTmpFile, err := runner.OutputCommand(ctx, true, "mktemp", "-u", "-p", "/var/tmp", "-t", "fwimgXXXXXX")
	if err != nil {
		return errors.Wrap(err, "creating remote temp file")
	}
	remoteImageFileName := strings.TrimSuffix(string(remoteTmpFile), "\n")
	defer runner.RunCommand(ctx, true, "rm", "-f", remoteImageFileName)

	if err = runner.PutFiles(ctx, true, map[string]string{localImageFileName: remoteImageFileName}); err != nil {
		return errors.Wrap(err, "could not copy files to servo host")
	}

	// -N == no verify all. It takes a long time to read the entire flashrom over CCD.
	args := []string{"-N", "-p", programmer, "-i", fmt.Sprintf("%s:%s", sec, remoteImageFileName), "-w"}
	args = append(args, extraFlashromArgs...)
	if err = runner.RunCommand(ctx, true, "flashrom", args...); err != nil {
		return errors.Wrap(err, "could not write host image")
	}

	return nil
}
