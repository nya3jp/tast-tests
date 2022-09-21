// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package env provides the basic building block in a virtualnet.
package env

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
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

	// Remove veth interface and the netns.
	if e.netnsCreated {
		if err := testexec.CommandContext(ctx, "ip", "netns", "del", e.NetNSName).Run(); err != nil {
			updateLastErrAndLog(errors.Wrapf(err, "failed to delete the netns %s", e.NetNSName))
		}
	}

	// Remove the chroot filesystem.
	if _, err := testexec.CommandContext(ctx, "rm", "-rf", "--one-file-system", e.chrootDir).Output(); err != nil {
		updateLastErrAndLog(errors.Wrap(err, "failed removing chroot filesystem"))
	}

	// Wait until veth pair is removed. It should happen once we remove the netns,
	// but it may take up to 2 seconds (on a local DUT) to finish.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := net.InterfaceByName(e.VethOutName); err == nil {
			return errors.Errorf("veth %s still exists", e.VethOutName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		updateLastErrAndLog(errors.Wrapf(err, "failed to wait for veth %s disappeared", e.VethOutName))
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

// IfaceAddrs represents the IP addresses configured on an interface.
type IfaceAddrs struct {
	// IPv4Addr is the IPv4 address on the interface. There is only one IPv4
	// address on an interface.
	IPv4Addr net.IP
	// IPv6Addrs is the list of IPv6 addresses (excluding link-local address) on
	// the interface.
	IPv6Addrs []net.IP
}

// All returns all addresses (excluding the IPv6 link-local address) on this
// interface.
func (addrs *IfaceAddrs) All() []net.IP {
	var ret []net.IP
	if addrs.IPv4Addr != nil {
		ret = append(ret, addrs.IPv4Addr)
	}
	return append(ret, addrs.IPv6Addrs...)
}

// GetVethInAddrs returns the current IP addresses configured on the veth
// interface inside this Env. Note that this function reads the addresses from
// the kernel directly, which also includes syscalls to change the netns, and
// thus it's better that the caller caches the results if possible. Note that if
// an address is configured dynamically (e.g., the IPv6 SLAAC address), it may
// not be ready immediately after the Env is ready (or the corresponding server
// starts).
func (e *Env) GetVethInAddrs(ctx context.Context) (retAddrs *IfaceAddrs, retErr error) {
	cleanup, err := e.EnterNetNS(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to enter the associated netns %s", e.NetNSName)
	}
	defer func() {
		if tempErr := cleanup(); tempErr != nil {
			testing.ContextLogf(ctx, "Failed to go back to the original netns from netns %s: %v", e.NetNSName, tempErr)
			if retErr == nil {
				retErr = tempErr
			}
		}
	}()
	iface, err := net.InterfaceByName(e.VethInName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interface object for the in interface")
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list addrs on the in interface")
	}

	// Each object in |addrs| implements the net.Addr interface, which is not very
	// easy to use. The following code converts it to a CIDR string and then a
	// net.IP object.
	var ret IfaceAddrs
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse CIDR string %s", addr)
		}
		if ipv4Addr := ip.To4(); ipv4Addr != nil {
			if ret.IPv4Addr != nil {
				return nil, errors.Errorf("there are two IPv4 addrs %s and %s on the in interface", ret.IPv4Addr, ipv4Addr)
			}
			ret.IPv4Addr = ipv4Addr
			continue
		}
		if ipv6Addr := ip.To16(); ipv6Addr != nil {
			if !ipv6Addr.IsLinkLocalUnicast() {
				ret.IPv6Addrs = append(ret.IPv6Addrs, ipv6Addr)
			}
			continue
		}
		return nil, errors.Wrapf(err, "%s is neither a v4 addr nor a v6 addr", ip)
	}
	return &ret, nil
}

// WaitForVethInAddrs polls the IP addresses on the inside interface and returns
// them until 1) there is IPv4 address if |ipv4| is true and 2) there is IPv6
// address (which is not a link-local address) if |ipv6| is true.
func (e *Env) WaitForVethInAddrs(ctx context.Context, ipv4, ipv6 bool) (*IfaceAddrs, error) {
	var addrs *IfaceAddrs
	if err := testing.Poll(ctx, func(c context.Context) error {
		var err error
		addrs, err = e.GetVethInAddrs(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get addrs of %s from env %s", e.VethInName, e.NetNSName)
		}
		if ipv4 && addrs.IPv4Addr == nil {
			return errors.Errorf("expect IPv4 addr in %v but was not found", addrs)
		}
		// Note that the link-local address not included in |addrs|.
		if ipv6 && len(addrs.IPv6Addrs) == 0 {
			return errors.Errorf("expect IPv6 addr in %v but was not found", addrs)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return nil, errors.Wrapf(err, "failed to wait for addrs in env %s", e.NetNSName)
	}
	return addrs, nil
}

// EnterNetNS executes the current OS thread in the netns associated with this
// Env. It returns the cleanup function, which switches the thread execution
// back to the original netns. Note that this function calls
// runtime.LockOSThread() to bind the calling goroutine to the current thread,
// since the netns only takes effect on a thread. The cleanup function MUST be
// called on the same goroutine with this function.
func (e *Env) EnterNetNS(ctx context.Context) (func() error, error) {
	// A helper function wraps the code to open the netns file with proper error
	// and log messages.
	openNSByPath := func(path string) (*os.File, func(), error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to open ns with path %s", path)
		}
		closeAndLogOnFailure := func() {
			if err := f.Close(); err != nil {
				testing.ContextLogf(ctx, "Failed to close file for ns %s: %v", path, err)
			}
		}
		return f, closeAndLogOnFailure, err
	}

	success := false

	runtime.LockOSThread()
	defer func() {
		if !success {
			runtime.UnlockOSThread()
		}
	}()

	// Open the current ns which will be used later in the cleanup closure.
	pid := unix.Getpid()
	tid := unix.Gettid()
	currentNSFile, currentNSClose, err := openNSByPath(fmt.Sprintf("/proc/%d/task/%d/ns/net", pid, tid))
	if err != nil {
		return nil, err
	}
	defer func() {
		if !success {
			currentNSClose()
		}
	}()

	// Open and enter the target ns.
	targetNSFile, targertNSClose, err := openNSByPath("/run/netns/" + e.NetNSName)
	if err != nil {
		return nil, err
	}
	defer targertNSClose()
	if err := unix.Setns(int(targetNSFile.Fd()), unix.CLONE_NEWNET); err != nil {
		return nil, errors.Wrapf(err, "failed to enter netns %s", e.NetNSName)
	}

	success = true
	return func() error {
		// File should always be closed.
		defer currentNSClose()

		tidNow := unix.Gettid()
		if tid != tidNow {
			return errors.Errorf("cleanup func does not run on the same thread as the one that enters the netns %s", e.NetNSName)
		}
		// Thread should be unlocked as long as we are on the same thread.
		defer runtime.UnlockOSThread()

		if err := unix.Setns(int(currentNSFile.Fd()), unix.CLONE_NEWNET); err != nil {
			return errors.Wrap(err, "failed to go back to the original netns")
		}
		return nil
	}, nil
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
	// Try to remove the leftover netns from the last test run if there is any.
	// This command will fail if the netns does not exist, which is also expected.
	if err := testexec.CommandContext(ctx, "ip", "netns", "del", e.NetNSName).Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return errors.Wrapf(err, "failed to delete leftover namespace %s", e.NetNSName)
		}
	}

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

	// Veth pair will be removed together with netns, so no explicit cleanup is
	// needed here.
	if err := testexec.CommandContext(ctx, "ip", "link",
		"add", e.VethOutName, "type", "veth",
		"peer", e.VethInName, "netns", e.NetNSName,
	).Run(); err != nil {
		return errors.Wrap(err, "failed to setup veth")
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

// ConnectToRouter connects this Env to |router| by moving the out interface
// into the netns of |router|, using |ipv4Subnet| and |ipv6Subnet|, configuring
// static IP addresses on both in and out interface, and installing routes for
// the subnet in both of the two netns. An additional default route will be
// added from this Env to |router|.
func (e *Env) ConnectToRouter(ctx context.Context, router *Env, ipv4Subnet, ipv6Subnet *net.IPNet) error {
	// Move the out interface into |router| and bring it up.
	if err := testexec.CommandContext(ctx, "ip", "link", "set", e.VethOutName, "netns", router.NetNSName).Run(); err != nil {
		return errors.Wrapf(err, "failed to move the out interface of %s into %s", e.NetNSName, router.NetNSName)
	}
	if err := router.RunWithoutChroot(ctx, "ip", "link", "set", e.VethOutName, "up"); err != nil {
		return errors.Wrapf(err, "failed to enable interface %s", e.VethOutName)
	}

	// Install IPv4 addresses and routes.
	ipv4Addr := ipv4Subnet.IP.To4()
	if ipv4Addr == nil {
		return errors.Errorf("invalid IPv4 subnet for connecting Envs: %v", ipv4Subnet)
	}
	selfIPv4Addr := net.IPv4(ipv4Addr[0], ipv4Addr[1], ipv4Addr[2], 2)
	routerIPv4Addr := net.IPv4(ipv4Addr[0], ipv4Addr[1], ipv4Addr[2], 1)
	if err := e.ConfigureInterface(ctx, e.VethInName, selfIPv4Addr, ipv4Subnet); err != nil {
		return errors.Wrapf(err, "failed to configure IPv4 on %s", e.VethInName)
	}
	if err := router.ConfigureInterface(ctx, e.VethOutName, routerIPv4Addr, ipv4Subnet); err != nil {
		return errors.Wrapf(err, "failed to configure IPv4 on %s", e.VethOutName)
	}
	if err := e.RunWithoutChroot(ctx, "ip", "route", "add", "default", "via", routerIPv4Addr.String()); err != nil {
		return errors.Wrap(err, "failed to add IPv4 default route")
	}

	// Install IPv6 addresses and routes.
	ipv6Addr := ipv6Subnet.IP.To16()
	if ipv6Addr == nil {
		return errors.Errorf("invalid IPv6 subnet for connecting Envs: %v", ipv6Subnet)
	}
	var selfIPv6Addr, routerIPv6Addr net.IP
	selfIPv6Addr = append([]byte{}, ipv6Addr...)
	selfIPv6Addr[15] = 2
	routerIPv6Addr = append([]byte{}, ipv6Addr...)
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
