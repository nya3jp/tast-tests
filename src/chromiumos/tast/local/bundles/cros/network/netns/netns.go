// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netns

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/veth"
	"chromiumos/tast/testing"
)

// Subset of bindRootDirectories that should be mounted writable.
var bindRootWritableDirectories = []string{"dev/pts"}

// Directories we'll bind mount when we want to bridge DBus namespaces.
// Includes directories containing the system bus socket and machine ID.
var dbusBridgeDirectories = []string{"run/dbus/", "var/lib/dbus/"}

var rootSymlinks = [][]string{{"var/run", "/run"}, {"var/lock", "/run/lock"}}

// NetNSEnv wraps the chroot variables.
type NetNSEnv struct {
	name        string
	NetNSName   string
	VethOutName string
	VethInName  string

	runningCmds []*testexec.Cmd

	netBindRootDirectories []string
	netRootDirectories     []string
	netTempDir             string
	netJailArgs            []string
	netnsCreated           bool
	vethPair               *veth.Pair
	NetEnv                 []string
}

// NewNetNSEnv creates a new chroot object.
func NewNetNSEnv(name string) *NetNSEnv {
	return &NetNSEnv{
		name:        name,
		NetNSName:   "netns-" + name,
		VethOutName: "etho_" + name,
		VethInName:  "ethi_" + name, // name should be short so that can be used as ifname

		netBindRootDirectories: []string{"bin", "dev", "dev/pts", "lib", "lib32", "lib64", "proc", "sbin", "sys", "usr", "usr/local", "usr/local/sbin"},
		netRootDirectories:     []string{"etc", "etc/ssl", "tmp", "var", "var/log", "run", "run/lock"},
	}
}

// Startup creates the chroot, calls patchpanel API to create a netns, starts
// user processes and returns the IPv4 address inside this netns.
func (n *NetNSEnv) Startup(ctx context.Context) (err error) {
	// Clean up if any step failed.
	defer func() {
		if err != nil {
			n.Shutdown(ctx)
		}
	}()

	if err := n.makeChroot(ctx); err != nil {
		return errors.Wrap(err, "failed to make the chroot")
	}

	if err := n.makeNetNS(ctx); err != nil {
		return errors.Wrap(err, "failed to create and connect to netns")
	}

	return nil
}

// Shutdown remove the chroot filesystem in which the VPN server was running.
func (n *NetNSEnv) Shutdown(ctx context.Context) error {
	// Stop running commands.
	for _, c := range n.runningCmds {
		testing.ContextLog(ctx, "Killing ", c.Path)
		c.Kill()
		c.DumpLog(ctx)
		c.Wait()
	}

	// Remove netns.
	if n.netnsCreated {
		if err := testexec.CommandContext(ctx, "ip", "netns", "del", n.NetNSName).Run(); err != nil {
			return errors.Wrapf(err, "failed to delete the netns %s", n.NetNSName)
		}
	}

	// Delete veth pair.
	if n.vethPair != nil {
		if err := n.vethPair.Delete(ctx); err != nil {
			return errors.Wrap(err, "failed to delete remove veth pair")
		}
	}

	// Remove the chroot filesystem.
	if _, err := testexec.CommandContext(ctx, "rm", "-rf", "--one-file-system", n.netTempDir).Output(); err != nil {
		return errors.Wrap(err, "failed removing chroot filesystem in which the VPN server was running")
	}

	return nil
}

// makeChroot makes a chroot filesystem.
func (n *NetNSEnv) makeChroot(ctx context.Context) error {
	temp, err := testexec.CommandContext(ctx, "mktemp", "-d", "/usr/local/tmp/chroot.XXXXXXXXX").Output()
	if err != nil {
		return errors.Wrap(err, "failed making temp directory: /usr/local/tmp/chroot.XXXXXXXXX")
	}
	n.netTempDir = strings.TrimSuffix(string(temp), "\n")
	if err := testexec.CommandContext(ctx, "chmod", "go+rX", n.netTempDir).Run(); err != nil {
		return errors.Wrapf(err, "failed to change mode to go+rX for the temp directory: %s", n.netTempDir)
	}

	// Make the root directories for the chroot.
	for _, rootdir := range n.netRootDirectories {
		if err := os.Mkdir(n.chrootPath(rootdir), os.ModePerm); err != nil {
			return errors.Wrap(err, "failed making the directory /run/shill")
		}
	}
	var srcPath string
	var dstPath string
	// Make the bind root driectories for the chroot.
	for _, rootdir := range n.netBindRootDirectories {
		srcPath = filepath.Join("/", rootdir)
		dstPath = n.chrootPath(rootdir)
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}
		if isLink(srcPath) {
			linkPath, err := os.Readlink(srcPath)
			if err != nil {
				return errors.Wrapf(err, "failed to readlink: %v", srcPath)
			}
			if err := os.Symlink(linkPath, dstPath); err != nil {
				return errors.Wrapf(err, "failed to Symlink %s to %s", linkPath, dstPath)
			}
		} else {
			if err := os.MkdirAll(dstPath, 0777); err != nil {
				return errors.Wrapf(err, "failed creating the directory: %s", dstPath)
			}
			mountArg := srcPath + "," + srcPath
			for _, dir := range bindRootWritableDirectories {
				if dir == rootdir {
					mountArg = mountArg + ",1"
				}
			}
			n.netJailArgs = append(n.netJailArgs, "-b", mountArg)
		}
	}

	for _, path := range rootSymlinks {
		srcPath = path[0]
		targetPath := path[1]
		linkPath := n.chrootPath(srcPath)
		if err := os.Symlink(targetPath, linkPath); err != nil {
			return errors.Wrapf(err, "failed to Symlink %s to %s", targetPath, linkPath)
		}
	}

	return nil
}

// makeNetNS ...
func (n *NetNSEnv) makeNetNS(ctx context.Context) error {
	var err error
	n.vethPair, err = veth.NewPair(ctx, n.VethInName, n.VethOutName)
	if err != nil {
		return errors.Wrap(err, "failed to setup veth")
	}

	// Create new namespace.
	if err := testexec.CommandContext(ctx, "ip", "netns", "add", n.NetNSName).Run(); err != nil {
		return errors.Wrapf(err, "failed to add the namespace %s", n.NetNSName)
	}
	n.netnsCreated = true

	// Move the in interface into the created netns.
	if err := testexec.CommandContext(ctx, "ip", "link", "set", n.VethInName, "netns", n.NetNSName).Run(); err != nil {
		return errors.Wrap(err, "failed to move the network interface to the namespace of the server")
	}

	if err := n.RunChroot(ctx, []string{"/bin/ip", "link", "set", n.VethInName, "up"}); err != nil {
		return errors.Wrap(err, "failed to enable interface")
	}

	if err := testexec.CommandContext(ctx, "ip", "netns", "exec", n.NetNSName, "sysctl", "-w", "net.ipv6.conf."+n.VethInName+".forwarding=1").Run(); err != nil {
		return errors.Wrap(err, "failed to move the network interface to the namespace of the server")
	}
	if err := testexec.CommandContext(ctx, "ip", "netns", "exec", n.NetNSName, "sysctl", "-w", "net.ipv6.conf.all.forwarding=1").Run(); err != nil {
		return errors.Wrap(err, "failed to move the network interface to the namespace of the server")
	}
	if err := testexec.CommandContext(ctx, "ip", "netns", "exec", n.NetNSName, "sysctl", "-w", "net.ipv6.conf."+n.VethInName+".accept_ra=2").Run(); err != nil {
		return errors.Wrap(err, "failed to move the network interface to the namespace of the server")
	}
	return nil
}

// chrootPath returns the the path within the chroot for |path|.
func (n *NetNSEnv) chrootPath(path string) string {
	return filepath.Join(n.netTempDir, strings.TrimLeft(path, "/"))
}

// RunChroot runs a command in a chroot, within the network namespace associated
// with this chroot.
func (n *NetNSEnv) RunChroot(ctx context.Context, args []string) error {
	minijailArgs := []string{"/sbin/minijail0", "-C", n.netTempDir}
	ipArgs := []string{"netns", "exec", n.NetNSName}
	ipArgs = append(ipArgs, minijailArgs...)
	ipArgs = append(ipArgs, n.netJailArgs...)
	ipArgs = append(ipArgs, args...)
	output, err := testexec.CommandContext(ctx, "ip", ipArgs...).CombinedOutput()
	o := string(output)
	if err != nil {
		return errors.Wrapf(err, "failed to run command inside the chroot: %s", o)
	}
	return nil
}

func (n *NetNSEnv) StartCommand(ctx context.Context, args ...string) error {
	minijailArgs := []string{"/sbin/minijail0", "-C", n.netTempDir}
	ipArgs := []string{"netns", "exec", n.NetNSName}
	ipArgs = append(ipArgs, minijailArgs...)
	ipArgs = append(ipArgs, n.netJailArgs...)
	ipArgs = append(ipArgs, args...)
	c := testexec.CommandContext(ctx, "ip", ipArgs...)
	if err := c.Start(); err != nil {
		return errors.Wrap(err, "failed to start command inside chroot")
	}
	n.runningCmds = append(n.runningCmds, c)
	return nil
}

func (n *NetNSEnv) WriteTempFile(ctx context.Context, name, contents string) error {
	if err := ioutil.WriteFile(n.chrootPath("tmp/"+name), []byte(contents), 0644); err != nil {
		return errors.Wrapf(err, "failed to write %s", name)
	}
	return nil
}

// getPidFile returns the integer contents of |pid_file| in the chroot.
func (n *NetNSEnv) getPidFile(pidFile string, missingOk bool) (int, error) {
	chrootPidFile := n.chrootPath(pidFile)
	content, err := ioutil.ReadFile(chrootPidFile)
	if err != nil {
		if !missingOk || !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}
		return 0, nil
	}

	intContent, err := strconv.Atoi(strings.TrimRight(string(content), "\n"))
	if err != nil {
		return 0, err
	}

	return intContent, nil
}

// KillPidFile kills the process belonging to |pid_file| in the chroot.
func (n *NetNSEnv) KillPidFile(ctx context.Context, pidFile string, missingOk bool) error {
	pid, err := n.getPidFile(pidFile, missingOk)
	if err != nil {
		return errors.Wrapf(err, "failed to get the pid for the file %s", pidFile)
	}
	if missingOk && pid == 0 {
		return nil
	}

	if err := testexec.CommandContext(ctx, "kill", fmt.Sprintf("%d", pid)).Run(); err != nil {
		return errors.Wrapf(err, "failed killing the pid %d", pid)
	}

	return nil
}

// GetLogContents return the logfiles from the chroot. |logFilePaths| is a list
// of relative paths to the chroot. An error will be returned if any file in the
// list does not exist.
func (n *NetNSEnv) GetLogContents(ctx context.Context, logFilePaths []string) (string, error) {
	var missingPaths []string

	headArgs := []string{"-10000"}
	for _, log := range logFilePaths {
		path := n.chrootPath(log)
		if assureExists(path) {
			headArgs = append(headArgs, path)
		} else {
			missingPaths = append(missingPaths, log)
		}
	}

	var contents string
	if len(headArgs) > 1 {
		output, err := testexec.CommandContext(ctx, "head", headArgs...).Output()
		if err != nil {
			return "", errors.Wrap(err, "failed getting the logfiles from the chroot")
		}
		contents = string(output)
	}

	if len(missingPaths) > 0 {
		return contents, errors.Errorf("files %v do not exist", missingPaths)
	}
	return contents, nil
}

// BridgeDbusNamespaces make the system DBus daemon visible inside the chroot.
func (n *NetNSEnv) BridgeDbusNamespaces() {
	n.netBindRootDirectories = append(n.netBindRootDirectories, dbusBridgeDirectories...)
}

// isLink returns path is a symbolic link.
func isLink(path string) bool {
	if !assureExists(path) {
		return false
	}

	fileInfoStat, err := os.Lstat(path)
	if err != nil {
		return false
	}

	if fileInfoStat.Mode()&os.ModeSymlink != os.ModeSymlink {
		return false
	}

	return true
}

// assureExists asserts that |path| exists.
func assureExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// copyFile copies file from source to destination.
func copyFile(srcFile, dstFile string) error {
	source, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}
	return nil
}
