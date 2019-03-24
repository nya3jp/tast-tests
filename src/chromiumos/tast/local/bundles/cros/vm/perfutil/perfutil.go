// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perfutil contains utilities needed for VM performance testing.
package perfutil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// ContainerHomeDir default home directory for test user in container.
const ContainerHomeDir = "/home/testuser"

// WriteError output errors to an io.Writer.
func WriteError(ctx context.Context, w io.Writer, title string, content []byte) {
	const logTemplate = "========== START %s ==========\n%s\n========== END ==========\n"
	if _, err := fmt.Fprintf(w, logTemplate, title, content); err != nil {
		testing.ContextLog(ctx, "Failed to write error to log file: ", err)
	}
}

// RunCmd runs cmd and returns its combined stdout and stderr.
// If an error is encountered, details are written to errWriter.
func RunCmd(ctx context.Context, cmd *testexec.Cmd, errWriter io.Writer) (out []byte, err error) {
	out, err = cmd.CombinedOutput()
	if err == nil {
		return out, nil
	}
	cmdString := strings.Join(append(cmd.Cmd.Env, cmd.Cmd.Args...), " ")

	// Write complete stdout and stderr to a log file.
	WriteError(ctx, errWriter, cmdString, out)

	// Only append the first and last line of the output to the error.
	out = bytes.TrimSpace(out)
	var errSnippet string
	if idx := bytes.IndexAny(out, "\r\n"); idx != -1 {
		lastIdx := bytes.LastIndexAny(out, "\r\n")
		errSnippet = fmt.Sprintf("%s ... %s", out[:idx], out[lastIdx+1:])
	} else {
		errSnippet = string(out)
	}
	return nil, errors.Wrap(err, errSnippet)
}

// ToTimeUnit returns time.Duration in unit as float64 numbers.
func ToTimeUnit(unit time.Duration, ts ...time.Duration) (out []float64) {
	for _, t := range ts {
		out = append(out, float64(t)/float64(unit))
	}
	return out
}

// parseLddOutput parses the output of a "ldd" command. Example "ldd" output:
//         libselinux.so.1 => /lib/x86_64-linux-gnu/libselinux.so.1 (0x00007f96da2cf000)
//         libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007f96d9f30000)
//         libpcre.so.3 => /lib/x86_64-linux-gnu/libpcre.so.3 (0x00007f96d9cbd000)
//         libdl.so.2 => /lib/x86_64-linux-gnu/libdl.so.2 (0x00007f96d9ab9000)
//         /lib64/ld-linux-x86-64.so.2 (0x00007f96da71a000)
//         libpthread.so.0 => /lib/x86_64-linux-gnu/libpthread.so.0 (0x00007f96d989c000)
func parseLddOutput(ctx context.Context, out string) (dynLibs map[string]string, dynLinker string) {
	dynLibPattern := regexp.MustCompile(`^(\S+) => (\S+)`)
	dynLinkerPattern := regexp.MustCompile(`^\S*lib\S*ld-linux\S*\.so\S*`)

	dynLibs = map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		matched := dynLibPattern.FindStringSubmatch(line)
		if matched != nil {
			testing.ContextLogf(ctx, "Found dynamic lib: %s", matched[0])
			dynLibs[matched[1]] = matched[2]
		} else {
			// Not a dynamic library line, checks if it's a dynamic linker line.
			matched := dynLinkerPattern.FindString(line)
			if matched != "" {
				testing.ContextLogf(ctx, "Found dynamic linker: %s", matched)
				if dynLinker != "" {
					testing.ContextLogf(ctx, "Already have %s, %s is ignored", dynLinker, matched)
					continue
				}
				dynLinker = matched
			}
		}
	}
	return dynLibs, dynLinker
}

// HostBinaryRunner runs a binary compiled for host in container. It is useful to ensure the same
// binary is used when the performance of the binary itself matters. This avoids performance gap
// introduced by different compiler/compiler flags/optimization level etc.
// It copies needed dynamic library and dynamic linker from host to container so the binary
// can be executed.
type HostBinaryRunner struct {
	binary                                          string        // Full path of the binary.
	cont                                            *vm.Container // A container instance to run in.
	contBinaryPath, contDynLinkerPath, contLibsPath string        // Used internally to specify file locations in container.
}

// NewHostBinaryRunner creates a HostBinaryRunner object.
func NewHostBinaryRunner(ctx context.Context, binary string, cont *vm.Container, errWriter io.Writer) (*HostBinaryRunner, error) {
	h := &HostBinaryRunner{binary: binary, cont: cont}

	baseName := filepath.Base(h.binary)
	// Copy the binary itself.
	testing.ContextLogf(ctx, "Copying %s to container", h.binary)
	h.contBinaryPath = filepath.Join(ContainerHomeDir, baseName)
	err := cont.PushFile(ctx, h.binary, h.contBinaryPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to copy %q to container", h.binary)
	}

	h.contLibsPath = filepath.Join(ContainerHomeDir, baseName+"_libs")
	_, err = RunCmd(ctx, cont.Command(ctx, "mkdir", "-p", h.contLibsPath), errWriter)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create %q in container", h.contLibsPath)
	}

	// Parse ldd output to get a list of dynamic libraries and dynamic linker.
	out, err := RunCmd(ctx, testexec.CommandContext(ctx, "ldd", h.binary), errWriter)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to run ldd on %q", h.binary)
	}
	dynLibs, dynLinker := parseLddOutput(ctx, string(out))
	if dynLinker == "" {
		return nil, errors.Errorf("could not find dynamic linker for %q", h.binary)
	}

	// Copy dynamic linker.
	testing.ContextLogf(ctx, "Copying dynamic linker %s to container", dynLinker)
	h.contDynLinkerPath = filepath.Join(ContainerHomeDir, filepath.Base(dynLinker))
	err = cont.PushFile(ctx, dynLinker, h.contDynLinkerPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to copy dynamic linker %q to container", dynLinker)
	}
	// Dynmic linker needs to be executable.
	_, err = RunCmd(ctx, cont.Command(ctx, "chmod", "755", h.contDynLinkerPath), errWriter)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to chmod %q", h.contDynLinkerPath)
	}
	// Copy dynamic libraries.
	for libName, libPath := range dynLibs {
		containerPath := filepath.Join(h.contLibsPath, libName)
		testing.ContextLogf(ctx, "Copying %s to %s", libPath, containerPath)
		err = cont.PushFile(ctx, libPath, containerPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to copy %q to %q", libPath, containerPath)
		}
	}
	return h, nil
}

// Command returns a command for executing the binary in the container
// with the supplied args.
func (h *HostBinaryRunner) Command(ctx context.Context, args ...string) *testexec.Cmd {
	cmdArgs := append([]string{h.contDynLinkerPath, "--library-path", h.contLibsPath, h.contBinaryPath}, args...)
	return h.cont.Command(ctx, cmdArgs...)
}
