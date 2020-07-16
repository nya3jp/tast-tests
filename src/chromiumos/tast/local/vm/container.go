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

	testContainerUsername = "testuser" // default container username during testing

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

// ContainerArchType represents the architecture+version of the container's image.
type ContainerArchType int

const (
	// DebianStretch refers to the "stretch" distribution of debian (a.k.a. debian 9).
	DebianStretch ContainerArchType = iota
	// DebianBuster refers to the "buster" distribution of debian (a.k.a. debian 10).
	DebianBuster
)

// ContainerType defines the type of container.
type ContainerType struct {
	// Image is the image source for this container.
	Image ContainerImageType
	// Arch is the architecture that the image has.
	Arch ContainerArchType
}

// Container encapsulates a container running in a VM.
type Container struct {
	// VM is the VM in which this container is running.
	VM                 *VM
	containerName      string // name of the container
	username           string // username of the container's primary user
	hostPrivateKey     string // private key to use when doing sftp to container
	containerPublicKey string // known_hosts for doing sftp to container
	ciceroneObj        dbus.BusObject
}

// locked is used to prevent creation of a container while the precondition is being used.
var locked = false

// prePackages lists packages containing preconditions that are allowed to call Lock and Unlock.
var prePackages = []string{
	"chromiumos/tast/local/crostini",
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

// newContainer returns a Container instance with a cicerone connection.
// Note that it assumes cicerone is up and running.
func newContainer(ctx context.Context, vmInstance *VM, containerName, userName string) (*Container, error) {
	c := Container{
		VM:            vmInstance,
		containerName: containerName,
		username:      userName,
	}
	if locked {
		panic("Do not create a new Container while a container precondition is active")
	}
	var err error
	if _, c.ciceroneObj, err = dbusutil.Connect(ctx, ciceroneName, ciceronePath); err != nil {
		return nil, err
	}
	return &c, nil
}

// DefaultContainer returns a container object with default settings.
func DefaultContainer(ctx context.Context, vmInstance *VM) (*Container, error) {
	return newContainer(ctx, vmInstance, DefaultContainerName, testContainerUsername)
}

// GetRunningContainer returns a Container struct for a currently running container.
// This is useful when the container was started by some other means, eg the installer.
// Will return an error if no container is currently running.
func GetRunningContainer(ctx context.Context, user string) (*Container, error) {
	vm, err := GetRunningVM(ctx, user)
	if err != nil {
		return nil, err
	}
	return DefaultContainer(ctx, vm)
}

// ArchitectureAlias returns the alias subpath of the chosen container
// architecture, i.e. part of the path used to compute the container's
// gsutil URL.
func ArchitectureAlias(t ContainerArchType) string {
	switch t {
	case DebianStretch:
		return "debian/stretch/test"
	default:
		return "debian/buster/test"
	}
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
		req.ImageAlias = ArchitectureAlias(t.Arch)
	case StagingImageServer:
		req.ImageServer = stagingContainerImageServerFormat
		req.ImageAlias = ArchitectureAlias(t.Arch)
	case Tarball:
		req.RootfsPath = tarballRootfsPath
		req.MetadataPath = tarballMetadataPath
	}

	resp := &cpb.CreateLxdContainerResponse{}
	if err := dbusutil.CallProtoMethod(ctx, c.ciceroneObj, ciceroneInterface+".CreateLxdContainer",
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
	if err := dbusutil.CallProtoMethod(ctx, c.ciceroneObj, ciceroneInterface+".StartLxdContainer",
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
		if err := c.DumpLog(ctx, dir); err != nil {
			testing.ContextLog(ctx, "Failure dumping container log: ", err)
		}
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
	if err := dbusutil.CallProtoMethod(ctx, c.ciceroneObj, ciceroneInterface+".GetLxdContainerUsername",
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
	if err := dbusutil.CallProtoMethod(ctx, c.ciceroneObj, ciceroneInterface+".SetUpLxdContainerUser",
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
		testContainerUsername + "@" + ip,
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

// LinuxPackageInfo queries the container for information about a Linux package
// file. The packageID returned corresponds to the package ID for an installed
// package based on the PackageKit specification which is of the form
// 'package_id;version;arch;repository'.
func (c *Container) LinuxPackageInfo(ctx context.Context, path string) (packageID string, err error) {
	resp := &cpb.LinuxPackageInfoResponse{}
	if err := dbusutil.CallProtoMethod(ctx, c.ciceroneObj, ciceroneInterface+".GetLinuxPackageInfo",
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
	if err = dbusutil.CallProtoMethod(ctx, c.ciceroneObj, ciceroneInterface+".InstallLinuxPackage",
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
	if err = dbusutil.CallProtoMethod(ctx, c.ciceroneObj, ciceroneInterface+".UninstallPackageOwningFile",
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
	// Add a dummy buffer for stdin to force allocating a pipe. vsh uses
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

// DumpLog dumps the logs from the container to a local output file named
// container_log.txt in dir (typically the test's output dir).
// It does this by executing croslog in the container and grabbing the output.
func (c *Container) DumpLog(ctx context.Context, dir string) error {
	f, err := os.Create(filepath.Join(dir, "container_log.txt"))
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := c.Command(ctx, "croslog", "--no-pager")
	cmd.Stdout = f
	return cmd.Run()
}

// CreateDefaultContainer prepares a container in VM with default settings.
// The vmInstance should be initialized and ready for a container to be created.
// The directory dir may be used to store logs on failure. If the container
// type is Tarball, then artifactPath must be specified with the path to the
// tarball containing the termina VM and container. Otherwise, artifactPath
// is ignored. Caller may wish to stop the VM if an error is returned.
func CreateDefaultContainer(ctx context.Context, vmInstance *VM, t ContainerType, dir string) (*Container, error) {
	created, err := dbusutil.NewSignalWatcherForSystemBus(ctx, ciceroneDBusMatchSpec("LxdContainerCreated"))
	if err != nil {
		return nil, err
	}
	// Always close the InstallLinuxPackageProgress watcher regardless of success.
	defer created.Close(ctx)

	c, err := DefaultContainer(ctx, vmInstance)
	if err != nil {
		return nil, err
	}
	if err := c.Create(ctx, t); err != nil {
		return nil, err
	}
	// Container is being created, wait for signal.
	createdSig := &cpb.LxdContainerCreatedSignal{}
	testing.ContextLogf(ctx, "Waiting for LxdContainerCreated signal for container %q, VM %q", c.containerName, vmInstance.name)
	if err := waitForDBusSignal(ctx, created, nil, createdSig); err != nil {
		return nil, errors.Wrap(err, "failed to get LxdContainerCreatedSignal")
	}
	if createdSig.GetVmName() != vmInstance.name {
		return nil, errors.Errorf("unexpected container creation signal for VM %q", createdSig.GetVmName())
	} else if createdSig.GetContainerName() != c.containerName {
		return nil, errors.Errorf("unexpected container creation signal for container %q", createdSig.GetContainerName())
	}
	if createdSig.GetStatus() != cpb.LxdContainerCreatedSignal_CREATED {
		return nil, errors.Errorf("failed to create container: status: %d reason: %v", createdSig.GetStatus(), createdSig.GetFailureReason())
	}

	if err := c.StartAndWait(ctx, dir); err != nil {
		return nil, err
	}
	return c, nil
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
		"/usr/lib/python3.7",
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
