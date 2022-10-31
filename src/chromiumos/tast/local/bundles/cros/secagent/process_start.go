// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package secagent tests security event reporting to missive.
package secagent

import (
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProcessStart,
		Desc: "Checks that process start events are correctly being reported",
		Contacts: []string{
			"jasonling@chromium.org",              // Test author
			"cros-enterprise-security@google.com", // Backup mailing list
		},
		Attr:    []string{"group:mainline", "informational"},
		Timeout: 3 * time.Minute,
	})
}

/*
	TODO(b/255624086): For now use our own structs but in the future

update missive so that we can use their protos directly
*/
type namespace struct {
	cgroup uint64
	ipc    uint64
	mnt    uint64
	net    uint64
	pid    uint64
	time   uint64
	user   uint64
	uts    uint64
}

type procInfo struct {
	pid     uint64
	ppid    uint64
	gid     uint64
	uid     uint64
	cmdline []string
	ns      namespace
}

func printNamespace(n *namespace) string {
	return fmt.Sprintf("namespace: cgroup:%d ipc:%d mnt:%d net:%d pid:%d time:%d user:%d uts:%d",
		n.cgroup, n.ipc, n.mnt, n.net, n.pid, n.time, n.user, n.uts)
}
func printCmdLine(c []string) string {
	return strings.Join(c, " ")
}

func printProcInfo(p *procInfo) string {
	return fmt.Sprintf("pid:%d ppid:%d gid:%d uid:%d\n cmdline:%s\n:%s",
		p.pid, p.ppid, p.gid, p.uid, strings.Join(p.cmdline[:], " "),
		printNamespace(&p.ns))
}

func parseNamespaces(p *procInfo, pid int, s *testing.State) error {

	r := regexp.MustCompile("(?m)^[a-z]+:\\[(?P<nsId>[[:digit:]]+)\\]")
	validNamespaces := map[string]*uint64{
		"cgroup": &p.ns.cgroup, "ipc": &p.ns.ipc, "mnt": &p.ns.mnt,
		"net": &p.ns.net, "pid": &p.ns.pid, "time": &p.ns.time,
		"user": &p.ns.user, "uts": &p.ns.uts}
	dirs, err := os.ReadDir(fmt.Sprintf("/proc/%d/ns", pid))
	if err != nil {
		return err
	}
	for _, f := range dirs {
		field, ok := validNamespaces[f.Name()]
		if ok == false {
			continue
		}
		nsSymlink := fmt.Sprintf("/proc/%d/ns/%s", pid, f.Name())
		rawNamespaceID, err := os.Readlink(nsSymlink)
		if err != nil {
			return errors.Wrapf(err, "Unable to read %s", nsSymlink)
		}
		m := r.FindStringSubmatch(rawNamespaceID)
		if m == nil {
			return errors.Errorf("Unrecognized namespace ID format %s in symlink %s", rawNamespaceID, nsSymlink)

		}
		nsID := m[r.SubexpIndex("nsId")]
		*field, err = strconv.ParseUint(nsID, 10, 64)
		if err != nil {
			return err
		}

	}
	return nil
}

func getCmdLine(pid int) ([]string, error) {
	cmdLine := fmt.Sprintf("/proc/%d/cmdline", pid)
	buff, err := ioutil.ReadFile(cmdLine)
	if err != nil {
		return []string{}, errors.Wrapf(err, "Unable to read %s", cmdLine)
	}
	return strings.Split(string(buff), string(rune(0))), nil
}

func parseCmdLine(p *procInfo, pid int, s *testing.State) error {
	var err error
	p.cmdline, err = getCmdLine(pid)
	return err
}

func parseProcStatus(p *procInfo, pid int, s *testing.State) error {
	r := regexp.MustCompile(
		"(?m)^Name:[[:blank:]]+(?P<name>[A-Za-z0-9_-]+)[[:blank:]]*[\\n\\r]+" +
			"^Umask:[[:blank:]]+(?P<umask>[[:digit:]]+)[[:blank:]]*[\\n\\r]+" +
			"^State:[[:blank:]]+(?P<state>[[:alpha:]]).*[\\n\\r]+" +
			"^Tgid:.*[\\n\\r]+" +
			"^Ngid:.*[\\n\\r]+" +
			"^Pid:[[:blank:]]+(?P<pid>[[:digit:]]+)[[:blank:]]*[\\n\\r]+" +
			"^PPid:[[:blank:]]+(?P<ppid>[[:digit:]]+)[[:blank:]]*[\\n\\r]+" +
			"^TracerPid:.*[\\n\\r]+" +
			"^Uid:[[:blank:]]+(?P<real_uid>[[:digit:]]+)[[:blank:]]*.*[\\n\\r]+" +
			"^Gid:[[:blank:]]+(?P<real_gid>[[:digit:]]+)[[:blank:]]*.*[\\n\\r]+")

	statusFilename := fmt.Sprintf("/proc/%d/status", pid)
	buff, err := ioutil.ReadFile(statusFilename)
	m := r.FindStringSubmatch(string(buff))
	if m == nil {
		return errors.Errorf("Unable to match {%s} using regex {%s}",
			string(buff), r.String())
	}
	p.ppid, err = strconv.ParseUint(m[r.SubexpIndex("ppid")], 10, 64)
	if err != nil {
		return err
	}
	p.gid, err = strconv.ParseUint(m[r.SubexpIndex("real_gid")], 10, 64)
	if err != nil {
		return err
	}
	p.uid, err = strconv.ParseUint(m[r.SubexpIndex("real_uid")], 10, 64)
	if err != nil {
		return err
	}
	return nil
}

func scrapeProcfs(pid int, s *testing.State) (procInfo, error) {
	var p procInfo
	var e error
	parsers := [3]func(*procInfo, int, *testing.State) error{parseProcStatus,
		parseCmdLine, parseNamespaces}
	for _, f := range parsers {
		if e = f(&p, pid, s); e != nil {
			return p, e
		}
	}
	return p, e
}

func startMissived(ctx context.Context, s *testing.State, testMode bool) error {
	testexec.CommandContext(ctx, "stop", "missived").Run()
	/* Currently missive does not have a test/debug mode.
	If test/debug mode becomes supported then run missive in this mode.
	Ideally, in this mode all messages would be dropped instead of being sent
	over ERP. For now Run() will likely fail so we don't check the error value.
	TODO(b/)
	*/
	var missiveStartCommand *testexec.Cmd
	if testMode {
		missiveStartCommand = testexec.CommandContext(ctx, "start", "missived")
	} else {
		missiveStartCommand = testexec.CommandContext(ctx, "start", "missived")
	}
	missiveStartCommand.Run()
	return nil
}

func startSecagentd(ctx context.Context, s *testing.State) (io.ReadCloser, error) {

	secagentdCmd := testexec.CommandContext(ctx, "minijail0", "-u secagentd",
		"-g secagentd", "-n", "-c", "cap_dac_read_search", "cap_sys_resource",
		"cap_perfmon", "cap_bpf", "cap_sys_ptrace=e",
		"--",
		"/usr/sbin/secagentd", "--log_level=0")
	stdOutPipe, err := secagentdCmd.StdoutPipe()
	_, err = os.Lstat("/etc/init/secagentd.conf")
	if err == nil {
		/* Detect whether an upstart config has been installed. If so prefer using
		upstart to restart the service. */
		/*TODO(jasonling): speak with Ryan and figure out what the upstart service name is
		then add a command to restart the service here
		*/
	} else {
		/* Upstart config is missing. This is expected during the early development of secagentd,
		kill secagentd if it is running and then restart it.
		*/
		testexec.CommandContext(ctx, "pkill", "secagentd").Run()
		/* copy pasted from src/platform2/secagentd/secagentd.conf:10810e8 */
		if err = secagentdCmd.Start(); err != nil {
			return stdOutPipe, errors.Wrap(err, "Unable to start secagentd")
		}
	}
	return stdOutPipe, nil
}

func ProcessStart(ctx context.Context, s *testing.State) {
	startMissived(ctx, s, true)
	/* Restart missive in non-test mode. */
	defer startMissived(ctx, s, false)
	_, err := startSecagentd(ctx, s)
	if err != nil {
		s.Fatalf("Secagentd could not be started successfully. error=%s", err)
	}
	obj, err := dbusutil.NewDBusObject(ctx, "org.freedesktop.DBus", "org.freedesktop.DBus",
		"/org/freedesktop/DBus")
	if err != nil {
		s.Fatalf("Unable to get dbus object with service name:"+
			"org.freedesktop.Dbus, path of /org/freedesktop/DBus and interface of "+
			"org.freedesktop.DBus:%s", err)
	}
	/* ListNames to get a list of dbus connections */
	var names []string
	if err = obj.Call(ctx, "ListNames").Store(&names); err != nil {
		s.Fatalf("Unable to retrieve a list of dbus connection names:%s", err)
	}
	var pid uint32
	var dbusConn string
	for _, name := range names {
		/* GetConnectionUnixProcessId to fetch the PID associated with each connection
		and then find the one that matches secagentd */
		if err = obj.Call(ctx, "GetConnectionUnixProcessID", name).Store(&pid); err != nil {
			s.Logf("Failed to get PID by Dbus for connection %s", name)
		}
		cmd, err := getCmdLine(int(pid))
		if err != nil {
			s.Logf("Unable to detect cmdline for dbus connection:%s pid:%d:%s",
				name, pid, err)
		} else {
			if strings.Trim(cmd[0], " ") == "/usr/sbin/secanomalyd" {
				dbusConn = name
				s.Logf("Detected /usr/sbin/secanomalyd connection name is %s", dbusConn)
				break
			}
		}
	}
	if dbusConn == "" {
		s.Fatal("Unable to determine the dbus connection name of /usr/sbin/secanomalyd")
	}

	/* Match rules so that we only monitor EnqueueRecord method
	calls originating from secagentd. */
	/* TODO: Currently this doesn't work. Introspecting dbus strangely shows
	that there are no methods/interfaces here..*/
	m := []dbusutil.MatchSpec{{Type: "method_call",
		Path:      dbus.ObjectPath("/org/chromium/Missived"),
		Interface: "org.chromium.Missived",
		Member:    "EnqueueRecord",
		Sender:    dbusConn}}
	/* copy-pasta from my investigations.
	   localhost ~ # dbus-send --system --dest=org.chromium.Missived --type=method_call --print-reply /org/chromium/Missived org.freedesktop.DBus.Introspectable.Introspect
	   method return time=1667178458.214949 sender=:1.106 -> destination=:1.138 serial=668 reply_serial=2
	      string "<!DOCTYPE node PUBLIC "-//freedesktop//DTD D-BUS Object Introspection 1.0//EN"
	   "http://www.freedesktop.org/standards/dbus/1.0/introspect.dtd">
	   <node>
	   </node>
	*/

	/* Start monitoring dbus. */
	stop, err := dbusutil.DbusEventMonitor(ctx, m)
	if err != nil {
		s.Fatalf("Unable to monitor dbus: %s", err)
	}

	/* Launch a sleep and scrape procfs and compare against what secagent
	reports via dbus */
	cmd := testexec.CommandContext(ctx, "sleep", "10")
	if err := cmd.Start(); err != nil {
		s.Fatalf("Error %s", err)
	}
	p, err := scrapeProcfs(cmd.Process.Pid, s)
	if err != nil {
		s.Fatalf("Error %s", err)
	}
	s.Logf("p=%s", printProcInfo(&p))

	/* Collect the log of EnqueueRecord dbus calls to Missived. */
	var methods []dbusutil.CalledMethod
	methods, err = stop()
	s.Logf("%d method calls detected", len(methods))
	if err != nil {
		s.Errorf("Failed to capture EnqueueRecord dbus calls to Missived: %s", err)
	}
	/* TODO: This is where we would take the details of the methods
	and convert the passed in parameters into protobuffs and compare them
	against what was scraped from procfs.
	*/
	for _, method := range methods {
		s.Logf("detected %s called", method)
	}
	if err = cmd.Kill(); err != nil {
		s.Logf("Failed to kill %s:%s", cmd.String(), err)
	}
	cmd.Wait()
}
