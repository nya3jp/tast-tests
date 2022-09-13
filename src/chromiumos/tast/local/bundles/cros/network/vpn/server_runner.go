// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vpn

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/testing"
)

// serverRunner is a helper struct to start a VPN server inside a virtualnet Env object.
type serverRunner struct {
	netRootDirectories     []string
	netConfigFileTemplates map[string]string
	netConfigFileValues    map[string]interface{}
	startupCmd             *testexec.Cmd
	NetEnv                 []string

	virtualNetEnv *env.Env
}

const (
	startup         = "etc/chroot_startup.sh"
	startupLog      = "var/log/startup.log"
	startupTemplate = "#!/bin/sh\n" +
		"exec > /{{.startup_log}} 2>&1\n" + // Redirect all commands output to the file startup.log.
		"set -x\n" // Print all executed commands to the terminal.
)

// newServerRunner creates a new serverRunner object.
func newServerRunner(virtualNetEnv *env.Env) *serverRunner {
	return &serverRunner{
		netConfigFileTemplates: map[string]string{startup: startupTemplate},
		netConfigFileValues:    map[string]interface{}{"startup_log": startupLog},
		virtualNetEnv:          virtualNetEnv,
	}
}

// Startup starts the VPN server in virtualNetEnv.
func (n *serverRunner) Startup(ctx context.Context) (string, error) {
	success := false
	defer func() {
		if success {
			return
		}
		if err := n.Shutdown(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to clean up serverRunner on setup failure: ", err)
		}
	}()

	addrs, err := n.virtualNetEnv.GetVethInAddrs(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get addrs from base env")
	}
	netnsIP := addrs.IPv4Addr.String()

	for _, rootdir := range n.netRootDirectories {
		if err := os.Mkdir(n.virtualNetEnv.ChrootPath(rootdir), os.ModePerm); err != nil {
			return "", errors.Wrapf(err, "failed to create root dir %s inside chroot", rootdir)
		}
	}

	n.netConfigFileValues["netns_ip"] = netnsIP
	if err := n.writeConfigs(); err != nil {
		return "", errors.Wrap(err, "failed writing the configs")
	}

	n.startupCmd = n.virtualNetEnv.CreateCommand(ctx, "/bin/bash", filepath.Join("/", startup))
	n.startupCmd.Env = append(os.Environ(), n.NetEnv...)
	if err := n.startupCmd.Start(); err != nil {
		return "", errors.Wrap(err, "failed to run minijail")
	}

	success = true
	return netnsIP, nil
}

// Shutdown stops the cmds running by this object.
func (n *serverRunner) Shutdown(ctx context.Context) error {
	// Wait for the startup command finishing. Kill it at first just in case if it is still running.
	if n.startupCmd != nil {
		n.startupCmd.Kill()
		n.startupCmd.Wait()
	}

	return nil
}

// assureExists asserts that |path| exists.
func assureExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// writeConfigs write config files.
func (n *serverRunner) writeConfigs() error {
	for configFile, fileTemplate := range n.netConfigFileTemplates {
		b := &bytes.Buffer{}
		template.Must(template.New("").Parse(fileTemplate)).Execute(b, n.netConfigFileValues)
		err := ioutil.WriteFile(n.virtualNetEnv.ChrootPath(configFile), []byte(b.String()), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

// RunChroot runs a command in a chroot, within the network namespace associated
// with this chroot.
func (n *serverRunner) RunChroot(ctx context.Context, args []string) error {
	output, err := n.virtualNetEnv.CreateCommand(ctx, args...).CombinedOutput()
	o := string(output)
	if err != nil {
		return errors.Wrapf(err, "failed to run command inside the chroot: %s", o)
	}
	return nil
}

// getPidFile returns the integer contents of |pid_file| in the chroot.
func (n *serverRunner) getPidFile(pidFile string, missingOk bool) (int, error) {
	chrootPidFile := n.virtualNetEnv.ChrootPath(pidFile)
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
func (n *serverRunner) KillPidFile(ctx context.Context, pidFile string, missingOk bool) error {
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
func (n *serverRunner) AddConfigTemplates(templates map[string]string) {
	for k, v := range templates {
		n.netConfigFileTemplates[k] = v
	}
}

// AddConfigValues add a name-value dict to the set of values for the config template.
func (n *serverRunner) AddConfigValues(values map[string]interface{}) {
	for k, v := range values {
		n.netConfigFileValues[k] = v
	}
}

// AddRootDirectories add |directories| to the set created within the chroot.
func (n *serverRunner) AddRootDirectories(directories []string) {
	n.netRootDirectories = append(n.netRootDirectories, directories...)
}

// AddStartupCommand add a command to the script run when the chroot starts up.
func (n *serverRunner) AddStartupCommand(command string) {
	n.netConfigFileTemplates[startup] = n.netConfigFileTemplates[startup] + fmt.Sprintf("%s\n", command)
}

// GetLogContents return the logfiles from the chroot. |logFilePaths| is a list
// of relative paths to the chroot. An error will be returned if any file in the
// list does not exist.
func (n *serverRunner) GetLogContents(ctx context.Context, logFilePaths []string) (string, error) {
	logFilePaths = append(logFilePaths, startupLog)
	var missingPaths []string

	headArgs := []string{"-10000"}
	for _, log := range logFilePaths {
		path := n.virtualNetEnv.ChrootPath(log)
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
