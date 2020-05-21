// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chroot implements a chroot environment that runs in a separate network namespace from the caller.
package chroot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

var bindRootDirectories = []string{"bin", "dev", "dev/pts", "lib", "lib32", "lib64", "proc", "sbin", "sys", "usr", "usr/local"}

// Subset of bindRootDirectories that should be mounted writable.
var bindRootWritableDirectories = []string{"dev/pts"}

// Directories we'll bind mount when we want to bridge DBus namespaces.
// Includes directories containing the system bus socket and machine ID.
var dbusBridgeDirectories = []string{"run/dbus/", "var/lib/dbus/"}

var rootDirectories = []string{"etc", "etc/ssl", "tmp", "var", "var/log", "run", "run/lock"}

var rootSymlinks = [][]string{{"var/run", "/run"}, {"var/lock", "/run/lock"}}

var copiedConfigFiles = []string{"etc/ld.so.cache"}

var configFileTemplates = map[string]string{
	startup: "#!/bin/sh\n" +
		"exec > /{{.startup_log}} 2>&1\n" + // Redirect all commands output to the file startup.log.
		"set -x\n", // Print all executed commands to the terminal.
}

// NetworkChroot wraps the chroot variables.
type NetworkChroot struct {
	netInterface           string
	netLocalIPandPrefix    string
	netBindRootDirectories []string
	netRootDirectories     []string
	netCopiedConfigFiles   []string
	netConfigFileTemplates map[string]string
	netConfigFileValues    map[string]string
	netTempDir             string
	netJailArgs            []string
}

const (
	startup    = "etc/chroot_startup.sh"
	startupLog = "var/log/startup.log"
	netnsVPN   = "netnsVPN"
)

// NewNetworkChroot creates a new chroot object.
func NewNetworkChroot(serverInterfaceName, serverAddress string, networkPrefix int) *NetworkChroot {
	tempConfigFileValues := make(map[string]string)
	tempConfigFileValues["local_ip"] = serverAddress
	tempConfigFileValues["startup_log"] = startupLog
	localIPandPrefix := fmt.Sprintf("%s/%d", serverAddress, networkPrefix)
	return &NetworkChroot{
		netInterface:           serverInterfaceName,
		netLocalIPandPrefix:    localIPandPrefix,
		netBindRootDirectories: bindRootDirectories,
		netRootDirectories:     rootDirectories,
		netCopiedConfigFiles:   copiedConfigFiles,
		netConfigFileTemplates: configFileTemplates,
		netConfigFileValues:    tempConfigFileValues}
}

// Startup create the chroot and start user processes.
func (n *NetworkChroot) Startup(ctx context.Context) error {
	if err := n.makeChroot(ctx); err != nil {
		return errors.Wrap(err, "failed making the chroot")
	}

	if err := n.writeConfigs(); err != nil {
		return errors.Wrap(err, "failed writing the configs")
	}

	// Create new namespace netnsVPN.
	if err := testexec.CommandContext(ctx, "ip", "netns", "add", netnsVPN).Run(); err != nil {
		return errors.Wrapf(err, "failed to add the namespace %s", netnsVPN)
	}

	// Move network interface to the network namespace of the server.
	if err := testexec.CommandContext(ctx, "ip", "link", "set", n.netInterface, "netns", netnsVPN).Run(); err != nil {
		return errors.Wrap(err, "failed to move the network interface to the namespace of the server")
	}

	if err := testexec.CommandContext(ctx, "ip", "-n", netnsVPN, "addr", "add", n.netLocalIPandPrefix, "dev", n.netInterface).Run(); err != nil {
		return errors.Wrapf(err, "failed to add the address %s to the server", n.netLocalIPandPrefix)
	}

	if err := testexec.CommandContext(ctx, "ip", "-n", netnsVPN, "link", "set", n.netInterface, "up").Run(); err != nil {
		return errors.Wrapf(err, "failed to set the network interface %s up", n.netInterface)
	}

	cmdArgs := append(n.netJailArgs, "/bin/bash", filepath.Join("/", startup), "&")
	ipArgs := []string{"netns", "exec", netnsVPN, "/sbin/minijail0", "-C", n.netTempDir}
	ipArgs = append(ipArgs, cmdArgs...)
	if err := testexec.CommandContext(ctx, "ip", ipArgs...).Start(); err != nil {
		return errors.Wrap(err, "failed to run minijail")
	}

	return nil
}

// Shutdown remove the chroot filesystem in which the VPN server was running.
func (n *NetworkChroot) Shutdown(ctx context.Context) error {
	// Delete the network namespace.
	if err := testexec.CommandContext(ctx, "ip", "netns", "del", netnsVPN).Run(); err != nil {
		return errors.Wrapf(err, "failed to delete the network namespace %s", netnsVPN)
	}

	if err := killPIDs(ctx, "/sbin/minijail0"); err != nil {
		testing.ContextLog(ctx, "Failed to get running pids inside /sbin/minijail0")
	}

	// Remove the chroot filesystem.
	if _, err := testexec.CommandContext(ctx, "rm", "-rf", "--one-file-system", n.netTempDir).Output(); err != nil {
		return errors.Wrap(err, "failed removing chroot filesystem in which the VPN server was running")
	}

	return nil
}

// killPIDs kills all PIDs in the execPath.
func killPIDs(ctx context.Context, execPath string) error {
	all, err := process.Pids()
	if err != nil {
		return errors.Wrap(err, "failed to get the running processes")
	}

	for _, pid := range all {
		proc, err := process.NewProcess(pid)
		if err != nil {
			// Assume that the process exited.
			continue
		} else if exe, err := proc.Exe(); err == nil && exe == execPath {
			if err := proc.Kill(); err != nil {
				testing.ContextLogf(ctx, "Failed to kill the process number %d", int(pid))
			}
		}
	}

	return nil
}

// makeChroot make a chroot filesystem.
func (n *NetworkChroot) makeChroot(ctx context.Context) error {
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

	for _, configFile := range n.netCopiedConfigFiles {
		srcPath = filepath.Join("/", configFile)
		dstPath = n.chrootPath(configFile)
		if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
			copyFile(srcPath, dstPath)
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

// chrootPath returns the the path within the chroot for |path|.
func (n *NetworkChroot) chrootPath(path string) string {
	return filepath.Join(n.netTempDir, strings.TrimLeft(path, "/"))
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

// writeConfigs write config files.
func (n *NetworkChroot) writeConfigs() error {
	for configFile, fileTemplate := range n.netConfigFileTemplates {
		b := &bytes.Buffer{}
		template.Must(template.New("").Parse(fileTemplate)).Execute(b, n.netConfigFileValues)
		err := ioutil.WriteFile(n.chrootPath(configFile), []byte(b.String()), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

// RunChroot run a command in a chroot, within a separate network namespace.
func (n *NetworkChroot) RunChroot(ctx context.Context, args []string) error {
	cmdArgs := append(n.netJailArgs, args...)
	minijailArgs := []string{"-e", "-C", n.netTempDir}
	minijailArgs = append(minijailArgs, cmdArgs...)
	if err := testexec.CommandContext(ctx, "minijail0", minijailArgs...).Start(); err != nil {
		return errors.Wrap(err, "failed to run command inside the chroot")
	}

	return nil
}

// getPidFile returns the integer contents of |pid_file| in the chroot.
func (n *NetworkChroot) getPidFile(pidFile string, missingOk bool) (int, error) {
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
func (n *NetworkChroot) KillPidFile(ctx context.Context, pidFile string, missingOk bool) error {
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

// AddConfigTemplates add a filename-content dict to the set of templates for the chroot.
func (n *NetworkChroot) AddConfigTemplates(templates map[string]string) {
	for k, v := range templates {
		n.netConfigFileTemplates[k] = v
	}
}

// AddConfigValues add a name-value dict to the set of values for the config template.
func (n *NetworkChroot) AddConfigValues(values map[string]string) {
	for k, v := range values {
		n.netConfigFileValues[k] = v
	}
}

// AddCopiedConfigFiles add |files| to the set to be copied to the chroot.
func (n *NetworkChroot) AddCopiedConfigFiles(files []string) {
	n.netCopiedConfigFiles = append(n.netCopiedConfigFiles, files...)
}

// AddRootDirectories add |directories| to the set created within the chroot.
func (n *NetworkChroot) AddRootDirectories(directories []string) {
	n.netRootDirectories = append(n.netRootDirectories, directories...)
}

// AddStartupCommand add a command to the script run when the chroot starts up.
func (n *NetworkChroot) AddStartupCommand(command string) {
	n.netConfigFileTemplates[startup] = n.netConfigFileTemplates[startup] + fmt.Sprintf("%s\n", command)
}

// GetLogContents return the logfiles from the chroot.
func (n *NetworkChroot) GetLogContents(ctx context.Context) (string, error) {
	startLog := n.chrootPath(startupLog)
	charonLog := n.chrootPath("var/log/charon.log")
	if assureExists(charonLog) && assureExists(startLog) {
		contents, err := testexec.CommandContext(ctx, "head", "-10000", charonLog, startLog).Output()
		if err != nil {
			return "", errors.Wrap(err, "failed getting the logfiles from the chroot")
		}
		return string(contents), nil
	} else if assureExists(charonLog) {
		testing.ContextLogf(ctx, "%s does not exist", startLog)
		contents, err := testexec.CommandContext(ctx, "head", "-10000", startLog).Output()
		if err != nil {
			return "", errors.Wrap(err, "failed getting the logfiles from the chroot")
		}
		return string(contents), nil
	} else if assureExists(startLog) {
		testing.ContextLogf(ctx, "%s does not exist", charonLog)
		contents, err := testexec.CommandContext(ctx, "head", "-10000", charonLog).Output()
		if err != nil {
			return "", errors.Wrap(err, "failed getting the logfiles from the chroot")
		}
		return string(contents), nil
	}

	return "", errors.Errorf("failed logfiles do not exist: %s, %s", startLog, charonLog)
}

// BridgeDbusNamespaces make the system DBus daemon visible inside the chroot.
func (n *NetworkChroot) BridgeDbusNamespaces() {
	n.netBindRootDirectories = append(n.netBindRootDirectories, dbusBridgeDirectories...)
}
