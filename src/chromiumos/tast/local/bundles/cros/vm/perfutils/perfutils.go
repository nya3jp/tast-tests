// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	containerHomeDir = "/home/testuser"
)

// WriteErrorProto a function proto for writing errors to a file.
type WriteErrorProto func(string, []byte)

// WriteErrorFunc returns a function which writes errors to a log file.:w
func WriteErrorFunc(ctx context.Context, errFile *os.File) WriteErrorProto {
	return func(title string, content []byte) {
		const logTemplate = "========== START %s ==========\n%s\n========== END ==========\n"
		if _, err := fmt.Fprintf(errFile, logTemplate, title, content); err != nil {
			testing.ContextLog(ctx, "Failed to write error to log file: ", err)
		}
	}
}

// RunCmdProto a function proto for running a command and handle errors.
type RunCmdProto func(cmd *testexec.Cmd) ([]byte, error)

// RunCmdFunc returns a function which runs a command and handle errors properly.
func RunCmdFunc(ctx context.Context, writeError WriteErrorProto) RunCmdProto {
	return func(cmd *testexec.Cmd) (out []byte, err error) {
		out, err = cmd.CombinedOutput()
		if err == nil {
			return out, nil
		}
		cmdString := strings.Join(append(cmd.Cmd.Env, cmd.Cmd.Args...), " ")
		if err := cmd.DumpLog(ctx); err != nil {
			testing.ContextLogf(ctx, "Failed to dump log for cmd %q: %v", cmdString, err)
		}

		// Write complete stdout and stderr to a log file.
		writeError(cmdString, out)

		// Only append the first and last line of the output to the error.
		out = bytes.TrimSpace(out)
		var errSnippet string
		if idx := bytes.IndexAny(out, "\r\n"); idx != -1 {
			lastIdx := bytes.LastIndexAny(out, "\r\n")
			errSnippet = fmt.Sprintf("%s ... %s", out[:idx], out[lastIdx+1:])
		} else {
			errSnippet = string(out)
		}
		return []byte{}, errors.Wrap(err, errSnippet)
	}
}

// ToTimeUnit returns time.Duration in unit |unit| as float64 numbers.
func ToTimeUnit(unit time.Duration, ts ...time.Duration) (out []float64) {
	for _, t := range ts {
		out = append(out, float64(t)/float64(unit))
	}
	return out
}

// HostBinaryRunner runs a binary compiled for host in container. It is useful to ensure the same binary is used
// when the performance of the binary itself matters. This avoids performance gap introduced
// by different compiler/compiler flags/optimization level etc.
// It copies needed dynamic library and dynamic linker from host to container so the binary
// can be executed.
type HostBinaryRunner struct {
	Binary                                          string      // Full path of the binary.
	RunCmd                                          RunCmdProto // A runCmdProto to execute commands.
	contBinaryPath, contDynLinkerPath, contLibsPath string      // Used internally to specify files locations container.
}

// Init setups needed files.
func (h *HostBinaryRunner) Init(ctx context.Context, cont *vm.Container) error {
	baseName := filepath.Base(h.Binary)

	// Copy the binary itself.
	testing.ContextLogf(ctx, "Copying %s to container", h.Binary)
	h.contBinaryPath = filepath.Join(containerHomeDir, baseName)
	err := cont.PushFile(ctx, h.Binary, h.contBinaryPath)
	if err != nil {
		return errors.Wrapf(err, "failed to copy %q to container", h.Binary)
	}

	// Copy dynamic libraries and dynamic linker.
	h.contLibsPath = filepath.Join(containerHomeDir, fmt.Sprintf("%s_libs", baseName))
	_, err = h.RunCmd(cont.Command(ctx, "mkdir", "-p", h.contLibsPath))
	if err != nil {
		return errors.Wrapf(err, "failed to create %q in container", h.contLibsPath)
	}

	// Use ldd and parse each line to handle dynamic libraries and dynamic linker.
	out, err := h.RunCmd(testexec.CommandContext(ctx, "ldd", h.Binary))
	if err != nil {
		return errors.Wrapf(err, "failed to run ldd on %q", h.Binary)
	}
	testing.ContextLog(ctx, string(out))
	dynLibPattern := regexp.MustCompile(`^(\S+) => (\S+)`)
	dynLinkerPattern := regexp.MustCompile(`^\S*lib\S*ld-linux\S*\.so\S*`)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		testing.ContextLog(ctx, line)
		matched := dynLibPattern.FindStringSubmatch(line)
		if matched != nil {
			testing.ContextLogf(ctx, "Found dynamic lib: %s", matched[0])
			libPath := matched[2]
			containerPath := filepath.Join(h.contLibsPath, matched[1])
			testing.ContextLogf(ctx, "Copying %s to %s", libPath, containerPath)
			err = cont.PushFile(ctx, libPath, containerPath)
			if err != nil {
				return errors.Wrapf(err, "failed to copy %q to %q", libPath, containerPath)
			}
		} else {
			// Possibly a dynamic linker line.
			dynLinker := dynLinkerPattern.FindString(line)
			if dynLinker != "" {
				testing.ContextLogf(ctx, "Copying dynamic linker %s to container", dynLinker)
				h.contDynLinkerPath = filepath.Join(containerHomeDir, filepath.Base(dynLinker))
				err = cont.PushFile(ctx, dynLinker, h.contDynLinkerPath)
				if err != nil {
					return errors.Wrapf(err, "failed to copy dynamic linker %q to container", dynLinker)
				}
				_, err = h.RunCmd(cont.Command(ctx, "chmod", "755", h.contDynLinkerPath))
				if err != nil {
					return errors.Wrapf(err, "failed to chmod %q", h.contDynLinkerPath)
				}
			}
		}
	}
	if h.contDynLinkerPath == "" {
		return errors.New("Could not find dynamic linker")
	}
	return nil
}

// Command returns a testexec.Cmd.
func (h *HostBinaryRunner) Command(ctx context.Context, cont *vm.Container, args ...string) *testexec.Cmd {
	cmdArgs := append([]string{h.contDynLinkerPath, "--library-path", h.contLibsPath, h.contBinaryPath}, args...)
	return cont.Command(ctx, cmdArgs...)
}
