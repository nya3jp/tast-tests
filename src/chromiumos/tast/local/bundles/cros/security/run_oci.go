// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RunOCI,
		Desc: "Verifies the functionality of the run_oci command",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"oci"},
	})
}

func RunOCI(ctx context.Context, s *testing.State) {
	defaultCfg := func() ociConfig {
		return ociConfig{
			OCIVersion: "1.0.0-rc1",
			Root:       ociRoot{Path: "rootfs", ReadOnly: true},
			Mounts: []ociMount{
				{Destination: "/", Type: "bind", Source: "/", Options: []string{"rbind", "ro"}},
				{Destination: "/proc", Type: "proc", Source: "proc", Options: []string{"nodev", "noexec", "nosuid"}},
			},
			Process:  ociProcess{Terminal: true, User: ociUser{UID: 0, GID: 0}, Cwd: "/"},
			Platform: ociPlatform{OS: "linux", Arch: "all"},
			Hostname: "runc",
			Linux: ociLinux{
				Namespaces: []ociNamespace{{Type: "cgroup"}, {Type: "pid"}, {Type: "network"}, {Type: "ipc"},
					{Type: "user"}, {Type: "uts"}, {Type: "mount"}},
				Resources: ociResources{
					Devices: []ociResourceDevice{
						{Allow: false, Access: "rwm"},
						{Allow: true, Type: "c", Major: 1, Minor: 5, Access: "r"},
					},
				},
				UIDMappings: []ociMapping{{HostID: sysutil.ChronosUID, ContainerID: 0, Size: 1}},
				GIDMappings: []ociMapping{{HostID: sysutil.ChronosGID, ContainerID: 0, Size: 1}},
			},
		}
	}

	type testCase struct {
		name       string               // short human-readable name for test case, e.g. "do-stuff"
		runOCIArgs []string             // additional top-level run_oci command-line args
		expStdout  string               // expected stdout from run_oci
		expStderr  string               // expected stderr from run_oci
		modifyCfg  func(cfg *ociConfig) // makes per-test modifications to default config
	}

	runTest := func(tc testCase) {
		// Create temp dir under /tmp to ensure that it's accessible by the chronos user.
		td, err := ioutil.TempDir("/tmp", "tast.security.RunOCI.")
		if err != nil {
			s.Fatal("Failed to create temp dir: ", err)
		}
		defer os.RemoveAll(td)
		if err := os.Chmod(td, 0755); err != nil {
			s.Fatal("Failed to chmod temp dir: ", err)
		}

		cfg := defaultCfg()
		tc.modifyCfg(&cfg)

		b, err := json.Marshal(&cfg)
		if err != nil {
			s.Fatal("Failed to marshal config to JSON: ", err)
		}
		cfgPath := filepath.Join(td, "config.json")
		if err := ioutil.WriteFile(cfgPath, b, 0644); err != nil {
			s.Fatal("Failed to create write file: ", err)
		}

		rootDir := filepath.Join(td, "rootfs")
		if err := os.Mkdir(rootDir, 0755); err != nil {
			s.Fatal("Failed to create root dir: ", err)
		}
		if err := os.Chown(rootDir, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
			s.Fatal("Failed to chown root dir: ", err)
		}

		var args []string
		args = append(args, "--cgroup_parent=chronos_containers")
		args = append(args, tc.runOCIArgs...)
		args = append(args, "run", "-c", td, "test_container")
		cmd := testexec.CommandContext(ctx, "/usr/bin/run_oci", args...)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		s.Logf("Case %v: running %v", tc.name, shutil.EscapeSlice(cmd.Args))
		cmd.Run() // ignore errors (many test cases intentionally run failing commands)

		failed := false
		if stdout.String() != tc.expStdout {
			failed = true
			s.Errorf("Case %v got stdout %q; want %q", tc.name, stdout.String(), tc.expStdout)
		}
		if stderr.String() != tc.expStderr {
			failed = true
			s.Errorf("Case %v got stderr %q; want %q", tc.name, stderr.String(), tc.expStderr)
		}
		if failed {
			// Unfortunately, we can't pass --log_dir to run_oci to tell it to write log messages
			// somewhere else where they could be reported by this test: doing so also causes the
			// container's stdout and stderr to be redirected to the file.
			s.Log("Check syslog for run_oci log messages")
		}
	}

	for _, tc := range []testCase{
		{
			name: "alt-syscall-settime",
			modifyCfg: func(cfg *ociConfig) {
				cfg.Process.Args = []string{"/bin/date", "-u", "--set", "010101"}
				cfg.Linux.AltSyscall = "third_party"
			},
			expStdout: "Mon Jan  1 00:00:00 UTC 2001\n",
			expStderr: "date: cannot set date: Function not implemented\n",
		},
		{
			name:       "bind-mount",
			runOCIArgs: []string{"--bind_mount=/bin:/var/log"},
			modifyCfg:  func(cfg *ociConfig) { cfg.Process.Args = []string{"/bin/ls", "/var/log/bash"} },
			expStdout:  "/var/log/bash\n",
		},
		{
			name: "device",
			modifyCfg: func(cfg *ociConfig) {
				cfg.Process.Args = []string{"/bin/ls", "/dev/null_test"}
				cfg.Mounts = append(cfg.Mounts, ociMount{Source: "tmpfs", Destination: "/dev", Type: "tmpfs", Options: []string{"noexec", "nosuid"}, PerformInIntermediateNamespace: true})
				cfg.Linux.Devices = append(cfg.Linux.Devices, ociLinuxDevice{Path: "/dev/null_test", Type: "c", Major: 1, Minor: 3, FileMode: 0666, UID: 0, GID: 0})
			},
			expStdout: "/dev/null_test\n",
		},
		{
			name:      "device-cgroup-allow",
			modifyCfg: func(cfg *ociConfig) { cfg.Process.Args = []string{"/usr/bin/hexdump", "-n16", "/dev/zero"} },
			expStdout: "0000000 0000 0000 0000 0000 0000 0000 0000 0000\n0000010\n",
		},
		{
			name:      "device-cgroup-deny",
			modifyCfg: func(cfg *ociConfig) { cfg.Process.Args = []string{"/usr/bin/hexdump", "-n1", "/dev/urandom"} },
			expStderr: "hexdump: /dev/urandom: Operation not permitted\nhexdump: all input file arguments failed\n",
		},
		{
			name:      "gid",
			modifyCfg: func(cfg *ociConfig) { cfg.Process.Args = []string{"/usr/bin/id", "-g"} },
			expStdout: "0\n",
		},
		{
			name: "hooks",
			modifyCfg: func(cfg *ociConfig) {
				cfg.Process.Args = []string{"/bin/echo", "-n", "3"}
				cfg.Hooks.PreChroot = append(cfg.Hooks.PreChroot, ociHook{Path: "/bin/echo", Args: []string{"echo", "-n", "0"}})
				cfg.Hooks.PreStart = append(cfg.Hooks.PreStart,
					ociHook{Path: "/bin/echo", Args: []string{"echo", "-n", "1"}},
					ociHook{Path: "/bin/echo", Args: []string{"echo", "-n", "2"}},
				)
				cfg.Hooks.PostStop = append(cfg.Hooks.PostStop, ociHook{Path: "/bin/echo", Args: []string{"echo", "-n", "4"}})
			},
			expStdout: "01234",
		},
		{
			name: "hooks-failure",
			modifyCfg: func(cfg *ociConfig) {
				cfg.Process.Args = []string{"/bin/echo", "-n", "This should not run"}
				cfg.Hooks.PreStart = append(cfg.Hooks.PreStart, ociHook{Path: "/bin/false", Args: []string{"false"}})
			},
		},
		{
			name:      "uid",
			modifyCfg: func(cfg *ociConfig) { cfg.Process.Args = []string{"/usr/bin/id", "-u"} },
			expStdout: "0\n",
		},
	} {
		runTest(tc)
	}
}

// ociConfig describes a JSON config file read by run_oci.
// See https://github.com/opencontainers/runtime-spec/blob/master/config.md and platform2/run_oci for details.
// This struct only contains configuration fields that are needed for this test.
type ociConfig struct {
	OCIVersion string      `json:"ociVersion"`
	Root       ociRoot     `json:"root,omitempty"`
	Mounts     []ociMount  `json:"mounts,omitempty"`
	Process    ociProcess  `json:"process,omitempty"`
	Platform   ociPlatform `json:"platform,omitempty"`
	Hostname   string      `json:"hostname,omitempty"`
	Hooks      ociHooks    `json:"hooks,omitempty"`
	Linux      ociLinux    `json:"linux,omitempty"`
}

// ociRoot describes the top-level "root" config entry.
type ociRoot struct {
	Path     string `json:"path"`
	ReadOnly bool   `json:"readonly,omitempty"`
}

// ociMount describes an entry in the top-level "mounts" config entry.
type ociMount struct {
	Destination string   `json:"destination"`
	Source      string   `json:"source,omitempty"`
	Type        string   `json:"type,omitempty"`
	Options     []string `json:"options,omitempty"`
	// PerformInIntermediateNamespace is a run_oci-specific extension.
	PerformInIntermediateNamespace bool `json:"performInIntermediateNamespace,omitempty"`
}

// ociProcess represents the top-level "process" config entry.
type ociProcess struct {
	Terminal bool     `json:"terminal,omitempty"`
	User     ociUser  `json:"user"`
	Args     []string `json:"args"`
	Cwd      string   `json:"cwd"`
}

// ociUser describes the "process.user" config entry.
type ociUser struct {
	UID int `json:"uid"`
	GID int `json:"gid"`
}

// ociPlatform represents the top-level "platform" config entry.
type ociPlatform struct {
	OS   string `json:"os,omitempty"`
	Arch string `json:"arch,omitempty"`
}

// ociHooks describes the top-level "hooks" config entry.
type ociHooks struct {
	// PreChroot is a run_oci-specific extension.
	PreChroot []ociHook `json:"prechroot"`
	PreStart  []ociHook `json:"prestart"`
	PostStop  []ociHook `json:"poststop"`
}

// ociHook describes an entry in e.g. "hooks.prechroot".
type ociHook struct {
	Path string   `json:"path"`
	Args []string `json:"args"`
}

// ociLinux describes the top-level "linux" config entry.
type ociLinux struct {
	Devices     []ociLinuxDevice `json:"devices"`
	Namespaces  []ociNamespace   `json:"namespaces"`
	Resources   ociResources     `json:"resources"`
	UIDMappings []ociMapping     `json:"uidMappings"`
	GIDMappings []ociMapping     `json:"gidMappings"`
	// AltSyscall is a run_oci-specific extension.
	AltSyscall string `json:"altSyscall,omitempty"`
}

// ociLinuxDevice describes an entry in "linux.devices".
type ociLinuxDevice struct {
	Path     string `json:"path"`
	Type     string `json:"type"`
	Major    int    `json:"major"`
	Minor    int    `json:"minor"`
	FileMode int    `json:"fileMode"`
	UID      int    `json:"uid"`
	GID      int    `json:"gid"`
}

// ociNamespace describes an entry in "linux.namespaces".
type ociNamespace struct {
	Type string `json:"type"`
}

// ociResources describes the "linux.resources" config entry.
type ociResources struct {
	Devices []ociResourceDevice `json:"devices"`
}

// ociResources describes an entry in "linux.resources.devices".
type ociResourceDevice struct {
	Allow  bool   `json:"allow"`
	Type   string `json:"type,omitempty"`
	Major  int    `json:"major,omitempty"`
	Minor  int    `json:"minor,omitempty"`
	Access string `json:"access,omitempty"`
}

// ociMapping describes an entry in "linux.uidMappings" or "linux.gidMappings".
type ociMapping struct {
	HostID      uint32 `json:"hostID"`
	ContainerID uint32 `json:"containerID"`
	Size        uint32 `json:"size"`
}
