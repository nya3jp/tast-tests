// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package env provides the basic building block in a virtualnet.
package env

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/veth"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/subnet"
	"chromiumos/tast/testing"
)

// maxNameLen is the limitation of the length of the name of a Env object. This
// limitation comes from the max ifname name length (IFNAMSIZ=16).
const maxNameLen = 10

var rootSymlinks = [][]string{{"var/run", "/run"}, {"var/lock", "/run/lock"}}

// bindRootDirs contains the paths which will be bind mounted when running a
// process.
var bindRootDirs = []string{"bin", "dev", "dev/pts", "etc/group", "etc/passwd", "lib", "lib32", "lib64", "proc", "sbin", "sys", "usr", "usr/local", "usr/local/sbin"}

// bindRootWritableDirs is the subset of bindRootDirs that should be mounted
// writable.
var bindRootWritableDirs = []string{"dev/pts"}

// createdRootDirs contains the paths which will be created inside the chroot.
var createdRootDirs = []string{"etc", "etc/ssl", "tmp", "var", "var/log", "run", "run/lock"}

// Env wraps the chroot variables.
type Env struct {
	name string

	// NetNSName is the name of netns associated with this object.
	NetNSName string
	// VethOutName is the name of the interface outside the associated netns.
	VethOutName string
	// VethInName is the name of the interface inside the associated netns.
	VethInName string

	chrootDir    string
	netJailArgs  []string
	netnsCreated bool
	servers      map[string]server
}

// A server represents a process (or processes for the same functionality)
// running in and managed by a Env. Struct that implements this interface can be
// registered with Env by StartServer(), and then when Env is shutting down,
// stop() and writeLogs() will be called to cleanup and collect logs.
type server interface {
	// Start starts the server.
	Start(ctx context.Context, e *Env) error
	// Stop stops the server.
	Stop(ctx context.Context) error
	// WriteLogs writes the logs with this server into |f|.
	WriteLogs(ctx context.Context, f *os.File) error
}

// New creates a new NewEnv object. It is caller's responsibility to call
// Cleanup() on the returned object if this call succeeded. |name| will be used
// as part of the names of netns, ifnames of veths, and the log file, and thus
// it should be unique among different Env objects.
func New(name string) (*Env, error) {
	if len(name) >= maxNameLen {
		return nil, errors.Errorf("the length of name %v is too long, should be shorter than %v", len(name), maxNameLen)
	}

	return &Env{
		name:        name,
		NetNSName:   "netns-" + name,
		VethOutName: "etho_" + name,
		VethInName:  "ethi_" + name,
		servers:     map[string]server{},
	}, nil
}

// SetUp starts the required environment, which includes a chroot, a netns, and
// a pair of veths with one peer inside the netns and the other peer outside the
// netns.
func (e *Env) SetUp(ctx context.Context) error {
	success := false
	defer func() {
		if success {
			return
		}
		if err := e.Cleanup(ctx); err != nil {
			testing.ContextLogf(ctx, "Failed to cleanup env %s: %v", e.name, err)
		}
	}()

	if err := e.makeChroot(ctx); err != nil {
		return errors.Wrap(err, "failed to make the chroot")
	}

	if err := e.makeNetNS(ctx); err != nil {
		return errors.Wrap(err, "failed to create and connect to netns")
	}

	success = true
	return nil
}

// Cleanup removes all the modifications that this object does on the DUT. The
// last error will be returned if any operation failed.
func (e *Env) Cleanup(ctx context.Context) error {
	var lastErr error

	updateLastErrAndLog := func(err error) {
		lastErr = err
		testing.ContextLog(ctx, "Cleanup failed: ", lastErr)
	}

	// Collect logs and clean up servers.
	f, err := e.createLogFile(ctx)
	if err != nil {
		updateLastErrAndLog(errors.Wrapf(err, "failed to open file for logging in %s", e.name))
	}
	for serverName, server := range e.servers {
		if err := server.Stop(ctx); err != nil {
			updateLastErrAndLog(errors.Wrapf(err, "failed to stop server %s in %s", serverName, e.name))
		}
		if f == nil {
			continue
		}
		if _, err := f.WriteString("\n\n>>>>> " + serverName + "\n"); err != nil {
			updateLastErrAndLog(errors.Wrapf(err, "failed to write header lines in log file for server %s in %s", serverName, e.name))
		}
		if err := server.WriteLogs(ctx, f); err != nil {
			updateLastErrAndLog(errors.Wrapf(err, "failed to write logs for server %s in %s", serverName, e.name))
		}
	}

	// Remove netns.
	if e.netnsCreated {
		if err := testexec.CommandContext(ctx, "ip", "netns", "del", e.NetNSName).Run(); err != nil {
			updateLastErrAndLog(errors.Wrapf(err, "failed to delete the netns %s", e.NetNSName))
		}
	}

	// Remove the chroot filesystem.
	if _, err := testexec.CommandContext(ctx, "rm", "-rf", "--one-file-system", e.chrootDir).Output(); err != nil {
		updateLastErrAndLog(errors.Wrap(err, "failed removing chroot filesystem"))
	}

	return lastErr
}

// StartServer starts a server inside this Env. This Env object will take care
// of the lifetime of the server.
func (e *Env) StartServer(ctx context.Context, name string, server server) error {
	if e.servers[name] != nil {
		return errors.Errorf("server with name %s already exists in %s", name, e.name)
	}
	e.servers[name] = server
	if err := server.Start(ctx, e); err != nil {
		return errors.Wrapf(err, "failed to start server %s", name)
	}
	return nil
}

func (e *Env) createLogFile(ctx context.Context) (*os.File, error) {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get ContextOutDir")
	}
	return os.OpenFile(filepath.Join(dir, e.name+"_logs.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

// makeChroot makes a chroot filesystem.
func (e *Env) makeChroot(ctx context.Context) error {
	temp, err := testexec.CommandContext(ctx, "mktemp", "-d", "/usr/local/tmp/chroot.XXXXXXXXX").Output()
	if err != nil {
		return errors.Wrap(err, "failed to make temp directory: /usr/local/tmp/chroot.XXXXXXXXX")
	}
	e.chrootDir = strings.TrimSuffix(string(temp), "\n")
	if err := testexec.CommandContext(ctx, "chmod", "go+rX", e.chrootDir).Run(); err != nil {
		return errors.Wrapf(err, "failed to change mode to go+rX for the temp directory: %s", e.chrootDir)
	}

	// Make the root directories for the chroot.
	for _, rootdir := range createdRootDirs {
		if err := os.Mkdir(e.ChrootPath(rootdir), os.ModePerm); err != nil {
			return errors.Wrapf(err, "failed to make the directory %s", rootdir)
		}
	}
	var srcPath, dstPath string
	// Make the bind root directories for the chroot.
	for _, rootdir := range bindRootDirs {
		srcPath = filepath.Join("/", rootdir)
		dstPath = e.ChrootPath(rootdir)
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
			mountArg := srcPath + "," + srcPath
			for _, dir := range bindRootWritableDirs {
				if dir == rootdir {
					mountArg = mountArg + ",1"
				}
			}
			e.netJailArgs = append(e.netJailArgs, "-b", mountArg)
		}
	}

	for _, path := range rootSymlinks {
		srcPath = path[0]
		targetPath := path[1]
		linkPath := e.ChrootPath(srcPath)
		if err := os.Symlink(targetPath, linkPath); err != nil {
			return errors.Wrapf(err, "failed to Symlink %s to %s", targetPath, linkPath)
		}
	}

	return nil
}

// makeNetNS prepares the veth pair and netns.
func (e *Env) makeNetNS(ctx context.Context) error {
	// Create new namespace.
	if err := testexec.CommandContext(ctx, "ip", "netns", "add", e.NetNSName).Run(); err != nil {
		return errors.Wrapf(err, "failed to add the namespace %s", e.NetNSName)
	}
	e.netnsCreated = true

	// Enable IP forwarding.
	if err := e.RunWithoutChroot(ctx, "sysctl", "-w", "net.ipv4.conf.all.forwarding=1"); err != nil {
		return errors.Wrapf(err, "failed to enable ipv4 forwarding in %s", e.NetNSName)
	}
	if err := e.RunWithoutChroot(ctx, "sysctl", "-w", "net.ipv6.conf.all.forwarding=1"); err != nil {
		return errors.Wrapf(err, "failed to enable ipv6 forwarding in %s", e.NetNSName)
	}

	vethPair, err := veth.NewPair(ctx, e.VethInName, e.VethOutName)
	if err != nil {
		return errors.Wrap(err, "failed to setup veth")
	}

	// Move the in interface into the created netns and bring it up.
	if err := testexec.CommandContext(ctx, "ip", "link", "set", e.VethInName, "netns", e.NetNSName).Run(); err != nil {
		// We only need to delete the veth pair if we failed to move one peer into
		// the netns. If this step succeeds, veth pair will be removed with the
		// netns together.
		if err := vethPair.Delete(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to delete veth pair: ", err)
		}
		return errors.Wrap(err, "failed to move the network interface to the namespace of the server")
	}

	if err := e.RunWithoutChroot(ctx, "ip", "link", "set", e.VethInName, "up"); err != nil {
		return errors.Wrapf(err, "failed to enable interface %s", e.VethInName)
	}

	return nil
}

// ChrootPath returns the the path within the chroot for |path|.
func (e *Env) ChrootPath(path string) string {
	return filepath.Join(e.chrootDir, strings.TrimLeft(path, "/"))
}

// RunWithoutChroot executes the command inside the netns but outside the
// chroot. Combined output will be wrapped in the error on failure. This is
// helpful when running command like `ip` and `sysctl`.
func (e *Env) RunWithoutChroot(ctx context.Context, args ...string) error {
	netnsArgs := []string{"netns", "exec", e.NetNSName}
	args = append(netnsArgs, args...)
	if o, err := testexec.CommandContext(ctx, "ip", args...).CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to run cmd in netns %s with output %s", e.NetNSName, string(o))
	}
	return nil
}

// CreateCommand creates a Cmd object which has the netns and chroot params
// configured. The caller should control the lifetime of this object.
func (e *Env) CreateCommand(ctx context.Context, args ...string) *testexec.Cmd {
	minijailArgs := []string{"/sbin/minijail0", "-C", e.chrootDir}
	ipArgs := []string{"netns", "exec", e.NetNSName}
	ipArgs = append(ipArgs, minijailArgs...)
	ipArgs = append(ipArgs, e.netJailArgs...)
	ipArgs = append(ipArgs, args...)
	return testexec.CommandContext(ctx, "ip", ipArgs...)
}

// ReadAndWriteLogIfExists reads the file contents from |path|, and writes them
// into |f|. It will not be treated as an error that the file does not exist
func (e *Env) ReadAndWriteLogIfExists(path string, f *os.File) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return errors.Wrapf(err, "failed to check existence of file %s", path)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", path)
	}

	if _, err := f.Write(b); err != nil {
		return errors.Wrapf(err, "failed to write contents of %s", path)
	}

	return nil
}

// ConnectToRouter connect this Env to |router| by moving the out interface into
// the netns of |router|, getting one IPv4 and one IPv6 subnet from |pool|,
// configuring static IP addresses on both in and out interface, and installing
// routes for the subnet in both of the two netns. An additional default route
// will be added from this Env to |router|.
func (e *Env) ConnectToRouter(ctx context.Context, router *Env, pool *subnet.Pool) error {
	// Move the out interface into |router| and bring it up.
	if err := testexec.CommandContext(ctx, "ip", "link", "set", e.VethOutName, "netns", router.NetNSName).Run(); err != nil {
		return errors.Wrap(err, "failed to move the network interface to the netns of router")
	}
	if err := router.RunWithoutChroot(ctx, "ip", "link", "set", e.VethOutName, "up"); err != nil {
		return errors.Wrapf(err, "failed to enable interface %s", e.VethOutName)
	}

	// Install IPv4 addresses and routes.
	ipv4Subnet, err := pool.AllocNextIPv4Subnet()
	if err != nil {
		return errors.Wrap(err, "failed to allocate IPv4 subnet for connecting Envs")
	}
	ipv4Addr := ipv4Subnet.IP.To4()
	selfIPv4Addr := net.IPv4(ipv4Addr[0], ipv4Addr[1], ipv4Addr[2], 2)
	routerIPv4Addr := net.IPv4(ipv4Addr[0], ipv4Addr[1], ipv4Addr[2], 1)
	if err := e.ConfigureInterface(ctx, e.VethInName, selfIPv4Addr, ipv4Subnet); err != nil {
		return errors.Wrapf(err, "failed to configure IPv4 on %s", e.VethInName)
	}
	if err := router.ConfigureInterface(ctx, e.VethOutName, routerIPv4Addr, ipv4Subnet); err != nil {
		return errors.Wrapf(err, "failed to configure IPv4 on %s", e.VethOutName)
	}
	if err := e.RunWithoutChroot(ctx, "ip", "route", "add", "default", "via", routerIPv4Addr.String()); err != nil {
		return errors.Wrap(err, "failed to add IPv4 defualt route")
	}

	// Install IPv6 addresses and routes.
	ipv6Subnet, err := pool.AllocNextIPv6Subnet()
	if err != nil {
		return errors.Wrap(err, "failed to allocate IPv6 subnet for connecting Envs")
	}
	var selfIPv6Addr, routerIPv6Addr net.IP
	selfIPv6Addr = append([]byte{}, ipv6Subnet.IP...)
	selfIPv6Addr[15] = 2
	routerIPv6Addr = append([]byte{}, ipv6Subnet.IP...)
	routerIPv6Addr[15] = 1
	if err := e.ConfigureInterface(ctx, e.VethInName, selfIPv6Addr, ipv6Subnet); err != nil {
		return errors.Wrapf(err, "failed to configure IPv6 on %s", e.VethInName)
	}
	if err := router.ConfigureInterface(ctx, e.VethOutName, routerIPv6Addr, ipv6Subnet); err != nil {
		return errors.Wrapf(err, "failed to configure IPv6 on %s", e.VethOutName)
	}
	if err := e.RunWithoutChroot(ctx, "ip", "route", "add", "default", "via", routerIPv6Addr.String()); err != nil {
		return errors.Wrap(err, "failed to add IPv6 default route")
	}

	return nil
}

// ConfigureInterface configures |addr| on |ifname|, and adds a route to point
// |subnet| to this interface.
func (e *Env) ConfigureInterface(ctx context.Context, ifname string, addr net.IP, subnet *net.IPNet) error {
	if err := e.RunWithoutChroot(ctx, "ip", "addr", "add", addr.String(), "dev", ifname); err != nil {
		return errors.Wrapf(err, "failed to install address %s on %s", addr.String(), ifname)
	}
	if err := e.RunWithoutChroot(ctx, "ip", "route", "add", subnet.String(), "dev", ifname); err != nil {
		return errors.Wrapf(err, "failed to install route %s on %s", subnet.String(), ifname)
	}
	testing.ContextLogf(ctx, "Installed %s with subnet %s on interface %s in netns %s", addr.String(), subnet.String(), ifname, e.NetNSName)
	return nil
}

// isLink returns whether path is a symbolic link.
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
