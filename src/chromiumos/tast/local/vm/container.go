// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus"

	cpb "chromiumos/system_api/vm_cicerone_proto" // protobufs for container management
	conciergepb "chromiumos/system_api/vm_concierge_proto"
	"chromiumos/tast/caller"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	liveContainerImageServerFormat    = "https://storage.googleapis.com/cros-containers/%d"         // simplestreams image server being served live
	stagingContainerImageServerFormat = "https://storage.googleapis.com/cros-containers-staging/%d" // simplestreams image server for staging
	tarballRootfsPath                 = "/mnt/shared/MyFiles/Downloads/crostini/container_rootfs.tar.xz"
	tarballMetadataPath               = "/mnt/shared/MyFiles/Downloads/crostini/container_metadata.tar.xz"

	ciceroneName      = "org.chromium.VmCicerone"
	ciceronePath      = dbus.ObjectPath("/org/chromium/VmCicerone")
	ciceroneInterface = "org.chromium.VmCicerone"
)

// ContainerImageType represents the mechanism/bucket that we should use to get the container.
type ContainerImageType int

const (
	// LiveImageServer indicates that the current live container image should be downloaded.
	LiveImageServer ContainerImageType = iota
	// StagingImageServer indicates that the current staging container image should be downloaded.
	StagingImageServer
	// Tarball indicates that the container image is available as tarball shared over 9P.
	Tarball
)

// ContainerDebianVersion represents the OS version of the container's image.
type ContainerDebianVersion string

const (
	// DebianStretch refers to the "stretch" distribution of debian (a.k.a. debian 9).
	DebianStretch ContainerDebianVersion = "stretch"
	// DebianBuster refers to the "buster" distribution of debian (a.k.a. debian 10).
	DebianBuster ContainerDebianVersion = "buster"
)

// ContainerArch represents the architecture of the container
type ContainerArch string

const (
	// Amd64 indicates that the container is built for amd64
	Amd64 ContainerArch = "amd64"
	// Arm indicates that the container is build for arm
	Arm ContainerArch = "arm"
)

// ContainerType defines the type of container.
type ContainerType struct {
	// Image is the image source for this container.
	Image ContainerImageType
	// DebianVersion is the version of debian that the image has.
	DebianVersion ContainerDebianVersion
}

// Container encapsulates a container running in a VM.
type Container struct {
	// VM is the VM in which this container is running.
	VM                 *VM
	containerName      string // name of the container
	username           string // username of the container's primary user
	hostPrivateKey     string // private key to use when doing sftp to container
	containerPublicKey string // known_hosts for doing sftp to container
}

// locked is used to prevent creation of a container while the precondition is being used.
var locked = false

// prePackages lists packages containing preconditions that are allowed to call Lock and Unlock.
var prePackages = []string{
	"chromiumos/tast/local/crostini",
	"chromiumos/tast/local/multivm",
}

// Lock prevents container creation/destruction until Unlock is called.
// It can only be called by preconditions and is idempotent.
func Lock() {
	caller.Check(2, prePackages)
	locked = true
}

// Unlock allows container creation after an earlier call to Lock.
// It can only be called by preconditions and is idempotent.
func Unlock() {
	caller.Check(2, prePackages)
	locked = false
}

// newContainer returns a Container instance.
// You must call Connect() to connect to the VM and Cicerone.
func newContainer(ctx context.Context, containerName, userName string) *Container {
	if locked {
		panic("Do not create a new Container while a container precondition is active")
	}
	return &Container{
		containerName: containerName,
		username:      userName,
	}
}

// DefaultContainer returns a container object with default settings.
func DefaultContainer(ctx context.Context, userEmail string) (*Container, error) {
	username := strings.SplitN(userEmail, "@", 2)[0]
	c := newContainer(ctx, DefaultContainerName, username)
	return c, c.Connect(ctx, userEmail)
}

// Connect connects the container to the running VM and cicerone instances.
func (c *Container) Connect(ctx context.Context, user string) error {
	vm, err := GetRunningVM(ctx, user)
	if err != nil {
		return err
	}
	c.VM = vm
	return nil
}

// GetRunningContainer returns a Container struct for a currently running container.
// This is useful when the container was started by some other means, eg the installer.
// Will return an error if no container is currently running.
func GetRunningContainer(ctx context.Context, user string) (*Container, error) {
	_, err := GetRunningVM(ctx, user)
	if err != nil {
		return nil, err
	}
	return DefaultContainer(ctx, user)
}

// ArchitectureAlias returns the alias subpath of the chosen container
// architecture, i.e. part of the path used to compute the container's
// gsutil URL.
func ArchitectureAlias(t ContainerDebianVersion) string {
	return fmt.Sprintf("debian/%s/test", t)
}

// Create will create a Linux container in an existing VM. It returns without waiting for the creation to complete.
// One must listen on cicerone D-Bus signals to know the creation is done.
func (c *Container) Create(ctx context.Context, t ContainerType) error {
	req := &cpb.CreateLxdContainerRequest{
		VmName:        c.VM.name,
		ContainerName: DefaultContainerName,
		OwnerId:       c.VM.Concierge.ownerID,
	}

	switch t.Image {
	case LiveImageServer:
		req.ImageServer = liveContainerImageServerFormat
		req.ImageAlias = ArchitectureAlias(t.DebianVersion)
	case StagingImageServer:
		req.ImageServer = stagingContainerImageServerFormat
		req.ImageAlias = ArchitectureAlias(t.DebianVersion)
	case Tarball:
		req.RootfsPath = tarballRootfsPath
		req.MetadataPath = tarballMetadataPath
	}

	resp := &cpb.CreateLxdContainerResponse{}
	if err := dbusutil.CallProtoMethod(ctx, c.VM.Concierge.ciceroneObj, ciceroneInterface+".CreateLxdContainer",
		req, resp); err != nil {
		return err
	}

	switch resp.GetStatus() {
	case cpb.CreateLxdContainerResponse_UNKNOWN, cpb.CreateLxdContainerResponse_FAILED:
		return errors.Errorf("failed to create container: %v", resp.GetFailureReason())
	case cpb.CreateLxdContainerResponse_EXISTS:
		return errors.New("container already exists")
	}
	return nil
}

// start launches a Linux container in an existing VM.
func (c *Container) start(ctx context.Context) error {
	starting, err := dbusutil.NewSignalWatcherForSystemBus(ctx, ciceroneDBusMatchSpec("LxdContainerStarting"))
	if err != nil {
		return err
	}
	// Always close the LxdContainerStarting watcher regardless of success.
	defer starting.Close(ctx)

	resp := &cpb.StartLxdContainerResponse{}
	if err := dbusutil.CallProtoMethod(ctx, c.VM.Concierge.ciceroneObj, ciceroneInterface+".StartLxdContainer",
		&cpb.StartLxdContainerRequest{
			VmName:        c.VM.name,
			ContainerName: c.containerName,
			OwnerId:       c.VM.Concierge.ownerID,
			Async:         true,
		}, resp); err != nil {
		return err
	}

	switch resp.GetStatus() {
	case cpb.StartLxdContainerResponse_RUNNING:
		return errors.New("container is already running")
	case cpb.StartLxdContainerResponse_STARTING, cpb.StartLxdContainerResponse_REMAPPING:
	default:
		return errors.Errorf("failed to start container: %v", resp.GetFailureReason())
	}

	sigResult := &cpb.LxdContainerStartingSignal{}
	for sigResult.VmName != c.VM.name ||
		sigResult.ContainerName != c.containerName ||
		sigResult.OwnerId != c.VM.Concierge.ownerID {
		if err := waitForDBusSignal(ctx, starting, nil, sigResult); err != nil {
			return err
		}
	}

	if sigResult.Status != cpb.LxdContainerStartingSignal_STARTED {
		return errors.Errorf("container failed to start: %v", resp.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Started container %q in VM %q", c.containerName, c.VM.name)
	return nil
}

// StartAndWait starts up an already created container and waits for that startup to complete
// before returning. The directory dir may be used to store logs on failure.
func (c *Container) StartAndWait(ctx context.Context, dir string) error {
	if err := c.SetUpUser(ctx); err != nil {
		return err
	}

	started, err := dbusutil.NewSignalWatcherForSystemBus(ctx, ciceroneDBusMatchSpec("ContainerStarted"))
	if err != nil {
		return err
	}
	// Always close the ContainerStarted watcher regardless of success.
	defer started.Close(ctx)

	if err = c.start(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Waiting for ContainerStarted D-Bus signal")
	sigResult := &cpb.ContainerStartedSignal{}
	for sigResult.VmName != c.VM.name ||
		sigResult.ContainerName != c.containerName ||
		sigResult.OwnerId != c.VM.Concierge.ownerID {
		if err := waitForDBusSignal(ctx, started, nil, sigResult); err != nil {
			return err
		}
	}
	return nil
}

// GetUsername returns the default user in a container.
func (c *Container) GetUsername(ctx context.Context) (string, error) {
	resp := &cpb.GetLxdContainerUsernameResponse{}
	if err := dbusutil.CallProtoMethod(ctx, c.VM.Concierge.ciceroneObj, ciceroneInterface+".GetLxdContainerUsername",
		&cpb.GetLxdContainerUsernameRequest{
			VmName:        c.VM.name,
			ContainerName: c.containerName,
			OwnerId:       c.VM.Concierge.ownerID,
		}, resp); err != nil {
		return "", err
	}

	if resp.GetStatus() != cpb.GetLxdContainerUsernameResponse_SUCCESS {
		return "", errors.Errorf("failed to get username: %v", resp.GetFailureReason())
	}

	return resp.GetUsername(), nil
}

// SetUpUser sets up the default user in a container.
func (c *Container) SetUpUser(ctx context.Context) error {
	resp := &cpb.SetUpLxdContainerUserResponse{}
	if err := dbusutil.CallProtoMethod(ctx, c.VM.Concierge.ciceroneObj, ciceroneInterface+".SetUpLxdContainerUser",
		&cpb.SetUpLxdContainerUserRequest{
			VmName:            c.VM.name,
			ContainerName:     c.containerName,
			OwnerId:           c.VM.Concierge.ownerID,
			ContainerUsername: c.username,
		}, resp); err != nil {
		return err
	}

	if resp.GetStatus() != cpb.SetUpLxdContainerUserResponse_SUCCESS &&
		resp.GetStatus() != cpb.SetUpLxdContainerUserResponse_EXISTS {
		return errors.Errorf("failed to set up user: %v", resp.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Set up user %q in container %q", c.username, c.containerName)
	return nil
}

// GetIPv4Address returns the IPv4 address of the container.
func (c *Container) GetIPv4Address(ctx context.Context) (ip string, err error) {
	cmd := c.Command(ctx, "hostname", "-I")
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to run %v", strings.Join(cmd.Args, " "))
	}
	return findIPv4(string(out))
}

func (c *Container) getContainerSSHKeys(ctx context.Context) error {
	if len(c.hostPrivateKey) > 0 && len(c.containerPublicKey) > 0 {
		return nil
	}
	_, conciergeObj, err := dbusutil.Connect(ctx, conciergeName, conciergePath)
	if err != nil {
		return err
	}
	resp := &conciergepb.ContainerSshKeysResponse{}
	if err := dbusutil.CallProtoMethod(ctx, conciergeObj, conciergeInterface+".GetContainerSshKeys",
		&conciergepb.ContainerSshKeysRequest{
			VmName:        c.VM.name,
			ContainerName: c.containerName,
			CryptohomeId:  c.VM.Concierge.ownerID,
		}, resp); err != nil {
		return err
	}

	c.hostPrivateKey = resp.HostPrivateKey
	c.containerPublicKey = resp.ContainerPublicKey
	return nil
}

// sftpCommand executes an SFTP command to perform a file transfer with the container.
// sftpCmd is any sftp command to be batch executed by sftp "-b" option.
func (c *Container) sftpCommand(ctx context.Context, sftpCmd string) error {
	if err := c.getContainerSSHKeys(ctx); err != nil {
		return errors.Wrap(err, "failed to get container ssh keys")
	}

	ip, err := c.GetIPv4Address(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get container IP address")
	}

	// Create temp dir to store sftp keys and command.
	// Though we can also pipe the commands to sftp via stdin, errors are not reflected on the
	// exit code of the sftp process. The exit code of "sftp -b" honors errors.
	dir, err := ioutil.TempDir("", "tast_vm_sftp_")
	if err != nil {
		return errors.Wrap(err, "failed to create temp dir for sftp keys and command")
	}
	defer os.RemoveAll(dir)
	privateKeyFile := filepath.Join(dir, "private_key")
	knownHostsFile := filepath.Join(dir, "known_hosts")
	cmdFile := filepath.Join(dir, "cmd")
	if err := ioutil.WriteFile(privateKeyFile, []byte(c.hostPrivateKey), 0600); err != nil {
		return errors.Wrap(err, "failed to write identity to temp file")
	}
	knownHosts := fmt.Sprintf("[%s]:2222 %s", ip, c.containerPublicKey)
	if err := ioutil.WriteFile(knownHostsFile, []byte(knownHosts), 0644); err != nil {
		return errors.Wrap(err, "failed to write known_hosts to temp file")
	}
	if err := ioutil.WriteFile(cmdFile, []byte(sftpCmd), 0644); err != nil {
		return errors.Wrap(err, "failed to write sftp command to temp file")
	}

	sftpArgs := []string{
		"-b", cmdFile,
		"-i", privateKeyFile,
		"-o", "UserKnownHostsFile=" + knownHostsFile,
		"-P", "2222",
		"-r",
		c.username + "@" + ip,
	}
	cmd := testexec.CommandContext(ctx, "sftp", sftpArgs...)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrapf(err, "failed to execute %q with sftp command %q", strings.Join(cmd.Args, " "), sftpCmd)
	}
	return nil
}

// PushFile copies a local file to the container's filesystem.
func (c *Container) PushFile(ctx context.Context, localPath, containerPath string) error {
	testing.ContextLogf(ctx, "Copying local file %v to container %v", localPath, containerPath)
	// Double quotes in sftp keeps spaces and invalidate special characters like * or ?.
	// Golang %q escapes " and \ and sftp unescape them correctly.
	// To handle a leading -, "--" is added after the command.
	putCmd := fmt.Sprintf("put -- %q %q", localPath, containerPath)
	return c.sftpCommand(ctx, putCmd)
}

// GetFile copies a remote file from the container's filesystem.
func (c *Container) GetFile(ctx context.Context, containerPath, localPath string) error {
	testing.ContextLogf(ctx, "Copying file %v from container %v", localPath, containerPath)
	// Double quotes in sftp keeps spaces and invalidate special characters like * or ?.
	// Golang %q escapes " and \ and sftp unescape them correctly.
	// To handle a leading -, "--" is added after the command.
	getCmd := fmt.Sprintf("get -- %q %q", containerPath, localPath)
	return c.sftpCommand(ctx, getCmd)
}

// CheckFilesExistInDir checks files exist in the given path in container.
// Returns error if any file does not exist or any other error.
func (c *Container) CheckFilesExistInDir(ctx context.Context, path string, files ...string) error {
	// Get file list in the path in container.
	fileList, err := c.GetFileList(ctx, path)
	if err != nil {
		return errors.Wrap(err, "failed to list the content of home directory in container")
	}

	// Create a map.
	set := make(map[string]struct{}, len(fileList))
	for _, s := range fileList {
		set[s] = struct{}{}
	}

	// Check each file exists in fileList.
	for _, file := range files {
		if _, ok := set[file]; !ok {
			return errors.Errorf("failed to find %s in container", file)
		}
	}
	return nil
}

// CheckFileDoesNotExistInDir checks files do not exist in the given path in container.
// Return error if any file exists or any other error.
func (c *Container) CheckFileDoesNotExistInDir(ctx context.Context, path string, files ...string) error {
	// Get file list in the path in container.
	fileList, err := c.GetFileList(ctx, path)
	if err != nil {
		return errors.Wrapf(err, "failed to list the content of %s in container", path)
	}

	// Create a map.
	set := make(map[string]struct{}, len(fileList))
	for _, s := range fileList {
		set[s] = struct{}{}
	}

	// Check each file does not exist in fileList.
	for _, file := range files {
		if _, ok := set[file]; ok {
			return errors.Errorf("unexpectedly found %s in container", file)
		}
	}
	return nil
}

// GetFileList returns a list of the files in the given path in the container.
func (c *Container) GetFileList(ctx context.Context, path string) (fileList []string, err error) {
	// Get files in the path in container.
	result, err := c.Command(ctx, "ls", "-1", path).Output()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to run 'ls %s' in container", path)
	}
	fileList = strings.Split(string(result), "\n")

	// Delete the last empty item if it is there.
	if len(fileList) > 0 && fileList[len(fileList)-1] == "" {
		fileList = fileList[:len(fileList)-1]
	}

	return fileList, nil
}

// CheckFileContent checks that the content of the specified file equals to the given string.
// Returns error if fail to read content or the contest does not equal to the given string.
func (c *Container) CheckFileContent(ctx context.Context, filePath, testString string) error {
	content, err := c.ReadFile(ctx, filePath)
	if err != nil {
		return errors.Wrapf(err, "failed to cat the result %s", filePath)
	}
	if content != testString {
		return errors.Wrapf(err, "want %s, got %q", testString, content)
	}
	return nil
}

// WriteFile creates a file in the container using echo.
func (c *Container) WriteFile(ctx context.Context, filePath, fileContent string) error {
	if err := c.Command(ctx, "sh", "-c", fmt.Sprintf("echo -n %s > %s", shutil.Escape(fileContent), filePath)).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to write file %v in container", filePath)
	}
	return nil
}

// RemoveAll removes a path from the container's file system using 'rm -rf'.
func (c *Container) RemoveAll(ctx context.Context, path string) error {
	return c.Command(ctx, "sudo", "rm", "-rf", path).Run(testexec.DumpLogOnError)
}

// ReadFile reads the content of file using command cat and returns it as a string.
func (c *Container) ReadFile(ctx context.Context, filePath string) (content string, err error) {
	cmd := c.Command(ctx, "cat", filePath)
	result, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to cat the content of %s", filePath)
	}

	return string(result), nil
}

// Cleanup removes all the files under the specific path.
func (c *Container) Cleanup(ctx context.Context, path string) error {
	list, err := c.GetFileList(ctx, path)
	if err != nil {
		return errors.Wrapf(err, "failed to get file list of %s in container: ", path)
	}
	for _, file := range list {
		if err := c.Command(ctx, "rm", "-rf", file).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to delete %s in %s", file, path)
		}
	}
	return nil
}

// LinuxPackageInfo queries the container for information about a Linux package
// file. The packageID returned corresponds to the package ID for an installed
// package based on the PackageKit specification which is of the form
// 'package_id;version;arch;repository'.
func (c *Container) LinuxPackageInfo(ctx context.Context, path string) (packageID string, err error) {
	resp := &cpb.LinuxPackageInfoResponse{}
	if err := dbusutil.CallProtoMethod(ctx, c.VM.Concierge.ciceroneObj, ciceroneInterface+".GetLinuxPackageInfo",
		&cpb.LinuxPackageInfoRequest{
			VmName:        c.VM.name,
			ContainerName: c.containerName,
			OwnerId:       c.VM.Concierge.ownerID,
			FilePath:      path,
		}, resp); err != nil {
		return "", err
	}

	if !resp.GetSuccess() {
		return "", errors.Errorf("failed to get Linux package info: %v", resp.GetFailureReason())
	}

	return resp.GetPackageId(), nil
}

// InstallPackage installs a Linux package file into the container.
func (c *Container) InstallPackage(ctx context.Context, path string) error {
	progress, err := dbusutil.NewSignalWatcherForSystemBus(ctx, ciceroneDBusMatchSpec("InstallLinuxPackageProgress"))
	if err != nil {
		return err
	}
	// Always close the InstallLinuxPackageProgress watcher regardless of success.
	defer progress.Close(ctx)

	resp := &cpb.InstallLinuxPackageResponse{}
	if err = dbusutil.CallProtoMethod(ctx, c.VM.Concierge.ciceroneObj, ciceroneInterface+".InstallLinuxPackage",
		&cpb.LinuxPackageInfoRequest{
			VmName:        c.VM.name,
			ContainerName: c.containerName,
			OwnerId:       c.VM.Concierge.ownerID,
			FilePath:      path,
		}, resp); err != nil {
		return err
	}

	if resp.Status != cpb.InstallLinuxPackageResponse_STARTED {
		return errors.Errorf("failed to start Linux package install: %v", resp.FailureReason)
	}

	// Wait for the signal for install completion which will signify success or
	// failure.
	testing.ContextLog(ctx, "Waiting for InstallLinuxPackageProgress D-Bus signal")
	sigResult := &cpb.InstallLinuxPackageProgressSignal{}
	for {
		if err := waitForDBusSignal(ctx, progress, nil, sigResult); err != nil {
			return err
		}
		if sigResult.VmName == c.VM.name &&
			sigResult.ContainerName == c.containerName &&
			sigResult.OwnerId == c.VM.Concierge.ownerID {
			if sigResult.Status == cpb.InstallLinuxPackageProgressSignal_SUCCEEDED {
				return nil
			}
			if sigResult.Status == cpb.InstallLinuxPackageProgressSignal_FAILED {
				return errors.Errorf("failure with Linux package install: %v", sigResult.FailureDetails)
			}
		}
	}
}

// UninstallPackageOwningFile uninstalls the package owning a particular desktop
// file in the container.
func (c *Container) UninstallPackageOwningFile(ctx context.Context, desktopFileID string) error {
	progress, err := dbusutil.NewSignalWatcherForSystemBus(ctx, ciceroneDBusMatchSpec("UninstallPackageProgress"))
	if err != nil {
		return err
	}
	// Always close the UninstallPackageProgress watcher regardless of success.
	defer progress.Close(ctx)

	resp := &cpb.UninstallPackageOwningFileResponse{}
	if err = dbusutil.CallProtoMethod(ctx, c.VM.Concierge.ciceroneObj, ciceroneInterface+".UninstallPackageOwningFile",
		&cpb.UninstallPackageOwningFileRequest{
			VmName:        c.VM.name,
			ContainerName: c.containerName,
			OwnerId:       c.VM.Concierge.ownerID,
			DesktopFileId: desktopFileID,
		}, resp); err != nil {
		return err
	}

	if resp.Status != cpb.UninstallPackageOwningFileResponse_STARTED {
		return errors.Errorf("failed to start package uninstall: %v", resp.FailureReason)
	}

	// Wait for the signal for uninstall completion which will signify success or
	// failure.
	testing.ContextLog(ctx, "Waiting for UninstallPackageProgress D-Bus signal")
	sigResult := &cpb.UninstallPackageProgressSignal{}
	for {
		if err := waitForDBusSignal(ctx, progress, nil, sigResult); err != nil {
			return err
		}
		if sigResult.VmName == c.VM.name && sigResult.ContainerName == c.containerName &&
			sigResult.OwnerId == c.VM.Concierge.ownerID {
			if sigResult.Status == cpb.UninstallPackageProgressSignal_SUCCEEDED {
				return nil
			}
			if sigResult.Status == cpb.UninstallPackageProgressSignal_FAILED {
				return errors.Errorf("failure with package uninstall: %v", sigResult.FailureDetails)
			}
		}
	}
}

// containerCommand returns a testexec.Cmd with a vsh command that will run in
// the specified container.
func containerCommand(ctx context.Context, vmName, containerName, ownerID string, vshArgs ...string) *testexec.Cmd {
	args := append([]string{"--vm_name=" + vmName,
		"--target_container=" + containerName,
		"--owner_id=" + ownerID,
		"--"},
		vshArgs...)
	cmd := testexec.CommandContext(ctx, "vsh", args...)
	// Add an empty buffer for stdin to force allocating a pipe. vsh uses
	// epoll internally and generates a warning (EPERM) if stdin is /dev/null.
	cmd.Stdin = &bytes.Buffer{}
	return cmd
}

// DefaultContainerCommand returns a testexec.Cmd with a vsh command that will run in
// the default termina/penguin container.
func DefaultContainerCommand(ctx context.Context, ownerID string, vshArgs ...string) *testexec.Cmd {
	return containerCommand(ctx, DefaultVMName, DefaultContainerName, ownerID, vshArgs...)
}

// Command returns a testexec.Cmd with a vsh command that will run in this
// container.
func (c *Container) Command(ctx context.Context, vshArgs ...string) *testexec.Cmd {
	return containerCommand(ctx, c.VM.name, c.containerName, c.VM.Concierge.ownerID, vshArgs...)
}

func ciceroneDBusMatchSpec(memberName string) dbusutil.MatchSpec {
	return dbusutil.MatchSpec{
		Type:      "signal",
		Path:      ciceronePath,
		Interface: ciceroneInterface,
		Member:    memberName,
	}
}

// ShrinkDefaultContainer deletes a lot of large files in the
// container to make the image size smaller.  This makes a big speed
// difference on slow devices for backup and restore.
func ShrinkDefaultContainer(ctx context.Context, ownerID string) error {
	// This list was constructed by running: `sudo du -b / | sort -n`,
	// and then deleting paths and checking that the container can still
	// be restarted.
	for _, path := range []string{
		"/usr/lib/gcc",
		"/usr/lib/git-core",
		"/usr/lib/python3",
		"/usr/share/doc",
		"/usr/share/fonts",
		"/usr/share/i18n",
		"/usr/share/icons",
		"/usr/share/locale",
		"/usr/share/man",
		"/usr/share/perl",
		"/usr/share/qt5",
		"/usr/share/vim",
		"/var/cache/apt",
		"/var/lib/apt",
		"/var/lib/dpkg",
	} {
		cmd := DefaultContainerCommand(ctx, ownerID, "sudo", "sh", "-c", "[ -e "+shutil.Escape(path)+" ]")
		if err := cmd.Run(); err != nil {
			return errors.Errorf("path %s does not exist", path)
		}
		cmd = DefaultContainerCommand(ctx, ownerID, "sudo", "rm", "-rf", path)
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

// ContainerCreationWatcher is a wrapper of SignalWatcher to trace container creation progress.
type ContainerCreationWatcher struct {
	cont    *Container
	watcher *dbusutil.SignalWatcher
}

// NewContainerCreationWatcher returns a ContainerCreationWatcher.
func NewContainerCreationWatcher(ctx context.Context, cont *Container) (*ContainerCreationWatcher, error) {
	watcher, err := dbusutil.NewSignalWatcherForSystemBus(ctx,
		ciceroneDBusMatchSpec("LxdContainerDownloading"), ciceroneDBusMatchSpec("LxdContainerCreated"))
	if err != nil {
		return nil, err
	}
	return &ContainerCreationWatcher{cont, watcher}, nil
}

// Close cleans up the SignalWatcher.
func (c *ContainerCreationWatcher) Close(ctx context.Context) {
	c.watcher.Close(ctx)
}

// isWatchingContainer returns whether the signal is for the container we are watching.
func (c *ContainerCreationWatcher) isWatchingContainer(vmName, containerName, ownerID string) bool {
	return vmName == c.cont.VM.name && containerName == c.cont.containerName && ownerID == c.cont.VM.Concierge.ownerID
}

// WaitForDownload waits for cicerone to send a container download notification.
// If pct is negative, this method returns after the next notification is received.
// Otherwise, it returns only after a notification with percent pct in [0, 100] is received.
// An error is returned if ctx's deadline is reached.
func (c *ContainerCreationWatcher) WaitForDownload(ctx context.Context, pct int32) error {
	spec := ciceroneDBusMatchSpec("LxdContainerDownloading")
	sigResult := &cpb.LxdContainerDownloadingSignal{}
	for {
		if err := waitForDBusSignal(ctx, c.watcher, &spec, sigResult); err != nil {
			return err
		}
		if c.isWatchingContainer(sigResult.VmName, sigResult.ContainerName, sigResult.OwnerId) {
			if pct < 0 || sigResult.DownloadProgress == pct {
				return nil
			}
		}
	}
}

// WaitForCreationComplete waits for the container to be created.
func (c *ContainerCreationWatcher) WaitForCreationComplete(ctx context.Context) error {
	spec := ciceroneDBusMatchSpec("LxdContainerCreated")
	sigResult := &cpb.LxdContainerCreatedSignal{}
	for {
		if err := waitForDBusSignal(ctx, c.watcher, &spec, sigResult); err != nil {
			return err
		}
		if c.isWatchingContainer(sigResult.VmName, sigResult.ContainerName, sigResult.OwnerId) {
			if sigResult.GetStatus() == cpb.LxdContainerCreatedSignal_CREATED {
				return nil
			}
		}
	}
}
