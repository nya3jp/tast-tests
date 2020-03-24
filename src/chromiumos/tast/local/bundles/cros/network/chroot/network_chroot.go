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
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

/*
	Implements a chroot environment that runs in a separate network
    namespace from the caller.  This is useful for network tests that
    involve creating a server on the other end of a virtual ethernet
    pair.  This object is initialized with an interface name to pass
    to the chroot, as well as the IP address to assign to this
    interface, since in passing the interface into the chroot, any
    pre-configured address is removed.

    The startup of the chroot is an orchestrated process where a
    small startup script is run to perform the following tasks:
      - Write out pid file which will be a handle to the
        network namespace that the |interface| should be passed to.
      - Wait for the network namespace to be passed in, by performing
        a "sleep" and writing the pid of this process as well.  Our
        parent will kill this process to resume the startup process.
      - We can now configure the network interface with an address.
      - At this point, we can now start any user-requested server
        processes.
*/

var bindRootDirectories = []string{"bin", "dev", "dev/pts", "lib", "lib32", "lib64", "proc", "sbin", "sys", "usr", "usr/local"}

// Subset of bindRootDirectories that should be mounted writable.
var bindRootWritableDirectories = []string{"dev/pts"}

// Directories we'll bind mount when we want to bridge DBus namespaces.
// Includes directories containing the system bus socket and machine ID.
var dbusBridgeDirectories = []string{"run/dbus/", "var/lib/dbus/"}

var rootDirectories = []string{"etc", "etc/ssl", "tmp", "var", "var/log", "run", "run/lock"}

var rootSymlinks = [][]string{{"var/run", "/run"}, {"var/lock", "/run/lock"}}

const (
	startup               = "etc/chroot_startup.sh"
	startupDelaySeconds   = "5"
	startupPidFile        = "run/vpn_startup.pid"
	startupSleeperPidFile = "run/vpn_sleeper.pid"
)

var copiedConfigFiles = []string{"etc/ld.so.cache"}

var configFileTemplates = map[string]string{
	startup: "#!/bin/sh\n" +
		"exec > /var/log/startup.log 2>&1\n" +
		"set -x\n" +
		"echo $$ > /{{.startup_pidfile}}\n" +
		"sleep {{.startup_delay_seconds}} &\n" +
		"echo $! > /{{.sleeper_pidfile}} &\n" +
		"wait\n" +
		"ip addr add {{.local_ip_and_prefix}} dev {{.local_interface_name}}\n" +
		"ip link set {{.local_interface_name}} up\n" +
		// For running strongSwan VPN with flag --with-piddir=/run/ipsec. We
		// want to use /run/ipsec for strongSwan runtime data dir instead of
		// /run, and the cmdline flag applies to both client and server.
		"mkdir -p /run/ipsec\n",
}

var configFileValues = map[string]string{
	"sleeper_pidfile":       startupSleeperPidFile,
	"startup_delay_seconds": startupDelaySeconds,
	"startup_pidfile":       startupPidFile,
}

var (
	tempDir  string
	jailArgs []string
)

// NetworkChroot wraps the chroot variables.
type NetworkChroot struct {
	netInterface           string
	netBindRootDirectories []string
	netRootDirectories     []string
	netCopiedConfigFiles   []string
	netConfigFileTemplates map[string]string
	netConfigFileValues    map[string]string
}

// NewNetworkChroot creates a new chroot object.
func NewNetworkChroot(serverInterfaceName string, serverAddress string, networkPrefix int) *NetworkChroot {
	tempConfigFileValues := configFileValues
	tempConfigFileValues["local_interface_name"] = serverInterfaceName
	tempConfigFileValues["local_ip"] = serverAddress
	addPrefix := serverAddress + "/" + fmt.Sprintf("%d", networkPrefix)
	tempConfigFileValues["local_ip_and_prefix"] = addPrefix
	return &NetworkChroot{netInterface: serverInterfaceName,
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

	if err := n.RunChroot(ctx, []string{"/bin/bash", filepath.Join("/", startup), "&"}); err != nil {
		return errors.Wrap(err, "failed eunning the chroot")
	}

	testing.ContextLog(ctx, "Waiting for 5sec")
	testing.Sleep(ctx, 5*time.Second)

	if err := n.moveInterfaceToChrootNamespace(ctx); err != nil {
		return errors.Wrap(err, "failed moving the interface to the chroot namespace")
	}

	if err := n.KillPidFile(ctx, startupSleeperPidFile, false); err != nil {
		return errors.Wrap(err, "failed killing the startup sleeper PID file")
	}

	return nil
}

// Shutdown remove the chroot filesystem in which the VPN server was running.
func (n *NetworkChroot) Shutdown(ctx context.Context) error {
	// TODO(pstew): Some processes take a while to exit, which will cause
	// the cleanup below to fail to complete successfully...
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		return err
	}

	_, err := testexec.CommandContext(ctx, "rm", "-rf", "--one-file-system", tempDir).Output()
	if err != nil {
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
	tempDir = strings.TrimSuffix(string(temp), "\n")
	if err := testexec.CommandContext(ctx, "chmod", "go+rX", tempDir).Run(); err != nil {
		return errors.Wrapf(err, "failed to change mode to go+rX for the temp directory: %s", tempDir)
	}

	for _, rootdir := range n.netRootDirectories {
		if err := os.Mkdir(chrootPath(rootdir), os.ModePerm); err != nil {
			return errors.Wrap(err, "failed making the directory /run/shill")
		}
	}
	var srcPath string
	var dstPath string
	for _, rootdir := range n.netBindRootDirectories {
		srcPath = filepath.Join("/", rootdir)
		dstPath = chrootPath(rootdir)
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
			jailArgs = append(jailArgs, "-b", mountArg)
		}
	}

	for _, configFile := range n.netCopiedConfigFiles {
		srcPath = filepath.Join("/", configFile)
		dstPath = chrootPath(configFile)
		if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
			copyFile(srcPath, dstPath)
		}
	}
	for _, path := range rootSymlinks {
		srcPath = path[0]
		targetPath := path[1]
		linkPath := chrootPath(srcPath)
		if err := os.Symlink(targetPath, linkPath); err != nil {
			return errors.Wrapf(err, "failed to Symlink %s to %s", targetPath, linkPath)
		}
	}

	return nil
}

// chrootPath returns the the path within the chroot for |path|.
func chrootPath(path string) string {
	return filepath.Join(tempDir, strings.TrimLeft(path, "/"))
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
func copyFile(srcFile string, dstFile string) error {
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
		err := ioutil.WriteFile(chrootPath(configFile), []byte(b.String()), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

// RunChroot run a command in a chroot, within a separate network namespace.
func (n *NetworkChroot) RunChroot(ctx context.Context, args []string) error {
	tempArgs := []string{"-e", "-C", tempDir}
	temp := append(jailArgs, args...)
	tempArgs = append(tempArgs, temp...)

	cmd := testexec.CommandContext(ctx, "minijail0", tempArgs...)
	cmd.Env = append(os.Environ())
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Start()
	if err != nil {
		return errors.Wrapf(err, "failed to create chroot %v/ Error = %s /output = %s", tempArgs, stderr.String(), out.String())
	}

	return nil
}

// moveInterfaceToChrootNamespace moves network interface to the network namespace of the server.
func (n *NetworkChroot) moveInterfaceToChrootNamespace(ctx context.Context) error {
	pid, err := getPidFile(startupPidFile, false)
	if err != nil {
		return errors.Wrapf(err, "failed to get the pid file of %s", startupPidFile)
	}

	cmd := testexec.CommandContext(ctx, "ip", "link", "set", n.netInterface, "netns", fmt.Sprintf("%d", pid))
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to move intf to chroot namespace /Error = %s /output = %s /tmpDir = %s", stderr.String(), out.String(), tempDir)
	}

	return nil
}

// getPidFile returns the integer contents of |pid_file| in the chroot.
func getPidFile(pidFile string, missingOk bool) (int, error) {
	chrootPidFile := chrootPath(pidFile)
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
	pid, err := getPidFile(pidFile, missingOk)
	if err != nil {
		return err
	}
	if missingOk && pid == 0 {
		return nil
	}

	if err := testexec.CommandContext(ctx, "kill", fmt.Sprintf("%d", pid)).Run(); err != nil {
		return err
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
	if !assureExists(chrootPath("var/log/*.log")) {
		testing.ContextLogf(ctx, "It doesn't exist %s", chrootPath("var/log/*.log"))
	}

	contents, err := testexec.CommandContext(ctx, "head", "-10000", chrootPath("var/log/charon.log"), chrootPath("var/log/startup.log")).Output()
	if err != nil {
		return "", errors.Wrap(err, "failed getting the logfiles from the chroot")
	}

	return string(contents), nil
}

// BridgeDbusNamespaces make the system DBus daemon visible inside the chroot.
func (n *NetworkChroot) BridgeDbusNamespaces() {
	n.netBindRootDirectories = append(n.netBindRootDirectories, dbusBridgeDirectories...)
}
