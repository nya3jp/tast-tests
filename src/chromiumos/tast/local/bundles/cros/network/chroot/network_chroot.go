// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chroot implements a chroot environment that runs in a separate network namespace from the caller.
package chroot

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	patchpanel "chromiumos/tast/local/network/patchpanel_client"
)

// Subset of bindRootDirectories that should be mounted writable.
var bindRootWritableDirectories = []string{"dev/pts"}

// Directories we'll bind mount when we want to bridge DBus namespaces.
// Includes directories containing the system bus socket and machine ID.
var dbusBridgeDirectories = []string{"run/dbus/", "var/lib/dbus/"}

var rootSymlinks = [][]string{{"var/run", "/run"}, {"var/lock", "/run/lock"}}

// NetworkChroot wraps the chroot variables.
type NetworkChroot struct {
	netBindRootDirectories []string
	netRootDirectories     []string
	netCopiedConfigFiles   []string
	netConfigFileTemplates map[string]string
	netConfigFileValues    map[string]interface{}
	netTempDir             string
	netJailArgs            []string
	netnsLifelineFD        *os.File
	netnsName              string
	startupCmd             *testexec.Cmd
	NetEnv                 []string
}

const (
	startup         = "etc/chroot_startup.sh"
	startupLog      = "var/log/startup.log"
	startupTemplate = "#!/bin/sh\n" +
		"exec > /{{.startup_log}} 2>&1\n" + // Redirect all commands output to the file startup.log.
		"set -x\n" // Print all executed commands to the terminal.
)

// NewNetworkChroot creates a new chroot object.
func NewNetworkChroot() *NetworkChroot {
	return &NetworkChroot{
		netBindRootDirectories: []string{"bin", "dev", "dev/pts", "lib", "lib32", "lib64", "proc", "sbin", "sys", "usr", "usr/local"},
		netRootDirectories:     []string{"etc", "etc/ssl", "tmp", "var", "var/log", "run", "run/lock"},
		netCopiedConfigFiles:   []string{"etc/ld.so.cache"},
		netConfigFileTemplates: map[string]string{startup: startupTemplate},
		netConfigFileValues:    map[string]interface{}{"startup_log": startupLog},
	}
}

// Startup creates the chroot, calls patchpanel API to create a netns, starts
// user processes and returns the IPv4 address inside this netns.
func (n *NetworkChroot) Startup(ctx context.Context) (string, error) {
	pc, err := patchpanel.New(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create patchpanel client")
	}

	// -1 indicates we want a new netns instead of naming a netns associated with a
	// process. We do not need to communicate outside the DUT in the VPN tests so
	// the other parameters do not matter.
	fd, resp, err := pc.ConnectNamespace(ctx, -1, "", false)
	if err != nil {
		return "", errors.Wrap(err, "failed to call ConnectNamespace")
	}
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(resp.PeerIpv4Address))
	netnsIP := net.IP(b).String()
	n.netConfigFileValues["netns_ip"] = netnsIP
	n.netnsLifelineFD = fd
	n.netnsName = resp.NetnsName

	if err := n.makeChroot(ctx); err != nil {
		return "", errors.Wrap(err, "failed making the chroot")
	}

	if err := n.writeConfigs(); err != nil {
		return "", errors.Wrap(err, "failed writing the configs")
	}

	cmdArgs := append(n.netJailArgs, "/bin/bash", filepath.Join("/", startup), "&")
	ipArgs := []string{"netns", "exec", n.netnsName, "/sbin/minijail0", "-C", n.netTempDir}
	ipArgs = append(ipArgs, cmdArgs...)
	n.startupCmd = testexec.CommandContext(ctx, "ip", ipArgs...)
	n.startupCmd.Env = append(os.Environ(), n.NetEnv...)
	if err := n.startupCmd.Start(); err != nil {
		return "", errors.Wrap(err, "failed to run minijail")
	}

	return netnsIP, nil
}

// Shutdown remove the chroot filesystem in which the VPN server was running.
func (n *NetworkChroot) Shutdown(ctx context.Context) error {
	// Delete the network namespace.
	if n.netnsLifelineFD != nil {
		n.netnsLifelineFD.Close()
	}

	// Wait for the startup command finishing. Kill it at first just in case if it is still running.
	if n.startupCmd != nil {
		n.startupCmd.Kill()
		n.startupCmd.Wait()
	}

	// Remove the chroot filesystem.
	if _, err := testexec.CommandContext(ctx, "rm", "-rf", "--one-file-system", n.netTempDir).Output(); err != nil {
		return errors.Wrap(err, "failed removing chroot filesystem in which the VPN server was running")
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

// RunChroot runs a command in a chroot, within a separate network namespace,
// and returns the output from stdout.
func (n *NetworkChroot) RunChroot(ctx context.Context, args []string) (string, error) {
	minijailArgs := []string{"/sbin/minijail0", "-C", n.netTempDir}
	ipArgs := []string{"netns", "exec", n.netnsName}
	ipArgs = append(ipArgs, minijailArgs...)
	ipArgs = append(ipArgs, n.netJailArgs...)
	ipArgs = append(ipArgs, args...)
	output, err := testexec.CommandContext(ctx, "ip", ipArgs...).Output()
	o := string(output)
	if err != nil {
		return o, errors.Wrapf(err, "failed to run command inside the chroot %s", o)
	}
	return o, nil
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
func (n *NetworkChroot) AddConfigValues(values map[string]interface{}) {
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

// GetLogContents return the logfiles from the chroot. |logFilePaths| is a list
// of relative paths to the chroot. A path will be ignored if it does not exist.
func (n *NetworkChroot) GetLogContents(ctx context.Context, logFilePaths []string) (string, error) {
	logFilePaths = append(logFilePaths, startupLog)

	headArgs := []string{"-10000"}
	for _, log := range logFilePaths {
		path := n.chrootPath(log)
		if assureExists(path) {
			headArgs = append(headArgs, path)
		}
	}

	if len(headArgs) == 1 {
		return "", nil
	}

	contents, err := testexec.CommandContext(ctx, "head", headArgs...).Output()
	if err != nil {
		return "", errors.Wrap(err, "failed getting the logfiles from the chroot")
	}

	return string(contents), nil
}

// BridgeDbusNamespaces make the system DBus daemon visible inside the chroot.
func (n *NetworkChroot) BridgeDbusNamespaces() {
	n.netBindRootDirectories = append(n.netBindRootDirectories, dbusBridgeDirectories...)
}
