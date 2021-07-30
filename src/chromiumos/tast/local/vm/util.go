// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
)

const (
	// TerminaComponentName is the name of the Chrome component for the VM kernel and rootfs.
	TerminaComponentName = "cros-termina"

	// TerminaMountDir is a path to the location where we will mount the termina component.
	TerminaMountDir = "/run/imageloader/cros-termina/99999.0.0"

	// ImageServerURLComponentName is the name of the Chrome component for the image server URL.
	ImageServerURLComponentName = "cros-crostini-image-server-url"

	lsbReleasePath = "/etc/lsb-release"
	milestoneKey   = "CHROMEOS_RELEASE_CHROME_MILESTONE"
)

// ComponentType represents the VM component type.
type ComponentType int

const (
	// ComponentUpdater indicates that the live component should be fetched from the component updater service.
	ComponentUpdater ComponentType = iota
	// StagingComponent indicates that the current staging component should be fetched from the GS component testing bucket.
	StagingComponent
)

// MountComponent mounts a component image from the provided image path.
func MountComponent(ctx context.Context, image string) error {
	if err := os.MkdirAll(TerminaMountDir, 0755); err != nil {
		return err
	}
	// Unmount any existing component.
	unix.Unmount(TerminaMountDir, 0)

	// We could call losetup manually and use the mount syscall... or
	// we could let mount(8) do the work.
	mountCmd := testexec.CommandContext(ctx, "mount", image, "-o", "loop", TerminaMountDir)
	if err := mountCmd.Run(); err != nil {
		mountCmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to mount component")
	}

	return nil
}

// UnmountComponent unmounts any active VM component.
func UnmountComponent(ctx context.Context) error {
	if err := unix.Unmount(TerminaMountDir, 0); err != nil {
		return errors.Wrap(err, "failed to unmount component")
	}

	if err := os.Remove(TerminaMountDir); err != nil {
		return errors.Wrap(err, "failed to remove component mount directory")
	}
	return nil
}

// getMilestone returns the Chrome OS milestone for this build.
func getMilestone() (int, error) {
	f, err := os.Open(lsbReleasePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := strings.Split(scanner.Text(), "=")
		if len(s) != 2 {
			continue
		}
		if s[0] == milestoneKey {
			val, err := strconv.Atoi(s[1])
			if err != nil {
				return 0, errors.Wrapf(err, "%q is not a valid milestone number", s[1])
			}
			return val, nil
		}
	}
	return 0, errors.New("no milestone key in lsb-release file")
}

// EnableCrostini sets the preference for Crostini being enabled as this is required for
// some of the Chrome integration tests to function properly.
func EnableCrostini(ctx context.Context, tconn *chrome.TestConn) error {
	if err := tconn.EvalPromiseDeprecated(ctx,
		`new Promise((resolve, reject) => {
		   chrome.autotestPrivate.setCrostiniEnabled(true, () => {
		     if (chrome.runtime.lastError === undefined) {
		       resolve();
		     } else {
		       reject(chrome.runtime.lastError.message);
		     }
		   });
		 })`, nil); err != nil {
		return errors.Wrap(err, "running autotestPrivate.setCrostiniEnabled failed")
	}
	return nil
}

// waitForDBusSignal waits on a SignalWatcher and returns the unmarshaled signal. optSpec matches a subset of the watching signals if watcher
// listens on multiple signals. Pass nil if we want to wait for any signal matches by watcher.
func waitForDBusSignal(ctx context.Context, watcher *dbusutil.SignalWatcher, optSpec *dbusutil.MatchSpec, sigResult proto.Message) error {
	for {
		select {
		case sig := <-watcher.Signals:
			if optSpec == nil || optSpec.MatchesSignal(sig) {
				if len(sig.Body) == 0 {
					return errors.New("signal lacked a body")
				}
				buf, ok := sig.Body[0].([]byte)
				if !ok {
					return errors.New("signal body is not a byte slice")
				}
				if err := proto.Unmarshal(buf, sigResult); err != nil {
					return errors.Wrap(err, "failed unmarshaling signal body")
				}
				return nil
			}
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "didn't get D-Bus signal")
		}
	}
}

// findIPv4 returns the first IPv4 address found in a space separated list of IPs.
func findIPv4(ips string) (string, error) {
	for _, v := range strings.Fields(ips) {
		ip := net.ParseIP(v)
		if ip != nil && ip.To4() != nil {
			return ip.String(), nil
		}
	}
	return "", errors.Errorf("could not find IPv4 address in %q", ips)
}

// RestartDefaultVMContainer restarts a VM and container that were previously shut down.
func RestartDefaultVMContainer(ctx context.Context, dir string, container *Container) error {
	if err := container.VM.Start(ctx); err != nil {
		return err
	}
	if err := container.StartAndWait(ctx, dir); err != nil {
		return err
	}
	return nil
}

// CreateVSHCommand creates a command to be run in a VM over vsh. The command
// parameter is required followed by an optional variatic list of strings as
// args. The command object is returned.
func CreateVSHCommand(ctx context.Context, cid int, command string, args ...string) *testexec.Cmd {
	params := append([]string{"--cid=" + strconv.Itoa(cid), "--", command}, args...)
	cmd := testexec.CommandContext(ctx, "vsh", params...)
	// Add an empty buffer for stdin to force allocating a pipe. vsh uses
	// epoll internally and generates a warning (EPERM) if stdin is /dev/null.
	cmd.Stdin = &bytes.Buffer{}
	return cmd
}
