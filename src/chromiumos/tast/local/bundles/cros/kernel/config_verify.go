// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConfigVerify,
		Desc: "Examines a kernel build CONFIG list to make sure various things are present, missing, built as modules, etc",
		Contacts: []string{
			"vapier@chromium.org",
			"chromeos-kernel@google.com",
			"oka@chromium.org", // Tast port author
		},
	})
}

func ConfigVerify(ctx context.Context, s *testing.State) {
	ver, arch, err := kernelVersionAndArch()
	if err != nil {
		s.Fatal("Failed to get kernel version and arch: ", err)
	}

	conf, err := readKernelConfig(ctx)
	if err != nil {
		s.Fatal("Failed to read kernel config: ", err)
	}

	newKernelConfigCheck(ver, arch).test(conf, s)
}

func kernelVersionAndArch() (*kernelVersion, string, error) {
	u, err := sysutil.Uname()
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to get uname")
	}
	t := strings.SplitN(u.Release, ".", 3)
	major, err := strconv.Atoi(t[0])
	if err != nil {
		return nil, "", errors.Wrapf(err, "wrong release format %q", u.Release)
	}
	minor, err := strconv.Atoi(t[1])
	if err != nil {
		return nil, "", errors.Wrapf(err, "wrong release format %q", u.Release)
	}
	return &kernelVersion{major: major, minor: minor}, u.Machine, nil
}

// readKernelConfig reads the kernel config key value pairs trimming CONFIG_ prefix from the keys.
func readKernelConfig(ctx context.Context) (map[string]string, error) {
	configs, err := readKernelConfigBytes(ctx)
	if err != nil {
		return nil, err
	}
	res := make(map[string]string)

	for _, line := range strings.Split(string(configs), "\n") {
		line := strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) < 2 || kv[1] == "" {
			return nil, errors.Errorf("unexpected config line %q", line)
		}
		const configPrefix = "CONFIG_"
		if !strings.HasPrefix(kv[0], configPrefix) {
			return nil, errors.Errorf("config %q doesn't start with %s unexpectedly", kv[0], configPrefix)
		}
		res[strings.TrimPrefix(kv[0], configPrefix)] = kv[1]
	}
	return res, nil
}

func readKernelConfigBytes(ctx context.Context) ([]byte, error) {
	const filename = "/proc/config.gz"
	// Load configs module to generate /proc/config.gz.
	if err := testexec.CommandContext(ctx, "modprobe", "configs").Run(); err != nil {
		return nil, errors.Wrap(err, "failed to generate kernel config file")
	}
	var r io.ReadCloser
	f, err := os.Open(filename)
	if err != nil {
		testing.ContextLogf(ctx, "Falling back: failed to open %s: %v", filename, err)
		u, err := sysutil.Uname()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get uname")
		}
		fallbackFile := "/boot/config-" + u.Release
		r, err = os.Open(fallbackFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open %s", fallbackFile)
		}
	} else { // Normal path.
		defer f.Close()
		r, err = gzip.NewReader(f)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create gzip reader for %s", filename)
		}
	}
	defer r.Close()
	configs, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config")
	}
	return configs, nil
}

type kernelVersion struct {
	major, minor int
}

func (v *kernelVersion) isOrLater(major, minor int) bool {
	return v.major > major || v.major == major && v.minor >= minor
}

// kernelConfigCheck contains configs to check.
type kernelConfigCheck struct {
	// exclusive contains regexes. The kernel config keys matching a regex should be listed in one of the following fields except missing. The keys are compared with removing the CONFIG_ prefix.
	exclusive []*regexp.Regexp
	// builtin contains keys that should be set to y.
	builtin []string
	// module contains keys that should be set to m.
	module []string
	// enabled contains keys that should be set to y or m.
	enabled []string
	// value contains key and value pairs that should be set.
	value map[string]string
	// same contains pairs of keys that should be set to the same value.
	// TODO(crbug/976562): In the original test, missing both keys were OK. Check the tastboard and see if we should mimic it. Otherwise remove this comment.
	same [][2]string
	// missing containing FOO indicates CONFIG_FOO shouldn't exist.
	missing []string
}

func newKernelConfigCheck(ver *kernelVersion, arch string) *kernelConfigCheck {
	exclusive := []*regexp.Regexp{
		// Security; no surprise binary formats.
		regexp.MustCompile(`^BINFMT_`),
		// Security; no surprise filesystem formats.
		regexp.MustCompile(`^.*_FS$`),
		// Security; no surprise partition formats.
		regexp.MustCompile(`^.*_PARTITION$`),
	}
	builtin := []string{
		// Sanity checks; should be present in builds as builtins.
		"INET",
		"MMU",
		"MODULES",
		"PRINTK",
		"SECURITY",
		// Security; enables the SECCOMP application API.
		"SECCOMP",
		// Security; blocks direct physical memory access.
		"STRICT_DEVMEM",
		// Security; provides some protections against SYN flooding.
		"SYN_COOKIES",
		// Security; make sure PID_NS, NET_NS, and USER_NS are enabled for
		// Chrome's layer 1 sandbox.
		"PID_NS",
		"NET_NS",
		"USER_NS",
		// Security; perform additional validation of credentials.
		"DEBUG_CREDENTIALS",
		// Security; make sure the Chrome OS LSM is in use.
		"SECURITY_CHROMIUMOS",

		// Binary formats.
		"BINFMT_ELF",

		// Filesystem formats.
		"DEBUG_FS",
		"ECRYPT_FS",
		"EXT4_FS",
		"PROC_FS",
		"SCSI_PROC_FS",
		"OVERLAY_FS",

		// Partition formats.
		"EFI_PARTITION",
		// MAC is for external drive formatted on Macintosh.
		"MAC_PARTITION",
		"MSDOS_PARTITION",
	}
	module := []string{
		// Sanity checks; should be present in builds as modules.
		"BLK_DEV_SR",
		"BT",
		"TUN",
		// Useful modules for users that should not be removed.
		"USB_SERIAL_OTI6858",

		"FAT_FS",
		"FUSE_FS",
		"HFSPLUS_FS",
		"ISO9660_FS",
		"UDF_FS",
		"VFAT_FS",
	}
	enabled := []string{
		// Either module or enabled, depending on platform.
		"VIDEO_V4L2",
	}
	value := map[string]string{
		// Security; NULL-address hole should be as large as possible.
		// Upstream kernel recommends 64k, which should be large enough
		// to catch nearly all dereferenced structures. For
		// compatibility with ARM binaries (even on x86) this needs to
		// be 32k.
		"DEFAULT_MMAP_MIN_ADDR": "32768",

		// NaCl; allow mprotect+PROT_EXEC on noexec mapped files.
		"MMAP_NOEXEC_TAINT": "0",
	}
	same := [][2]string{}
	missing := []string{
		// Never going to optimize to this CPU.
		"M386",
		// Config not in real kernel config var list.
		"CHARLIE_THE_UNICORN",
		// Dangerous; allows direct physical memory writing.
		"ACPI_CUSTOM_METHOD",
		// Dangerous; disables brk(2) ASLR.
		"COMPAT_BRK",
		// Dangerous; disables VDSO ASLR.
		"COMPAT_VDSO",
		// Dangerous; allows direct kernel memory writing.
		"DEVKMEM",
		// Dangerous; allows replacement of running kernel.
		"KEXEC",
		// Dangerous; allows replacement of running kernel.
		"HIBERNATION",
		// We don't need to provide access to *all* symbols in /proc/kallsyms.
		"KALLSYMS_ALL",
		// This callback can be subverted to point to arbitrary programs.  We
		// require firmware to be in the rootfs at normal locations which lets
		// the kernel locate things itself.
		"FW_LOADER_USER_HELPER",
		"FW_LOADER_USER_HELPER_FALLBACK",

		// Sanity checks (binfmt); one disabled, one does not exist.
		"BINFMT_AOUT",
		"BINFMT_IMPOSSIBLE",

		// Sanity checks (fs); ones disabled, one does not exist.
		"EXT2_FS",
		"EXT3_FS",
		"XFS_FS",
		"IMPOSSIBLE_FS",

		// Sanity checks (partition); one disabled, one does not exist.
		"LDM_PARTITION",
		"IMPOSSIBLE_PARTITION",
	}

	if ver.isOrLater(3, 10) {
		builtin = append(builtin, "BINFMT_SCRIPT")
	}

	if ver.isOrLater(3, 14) {
		builtin = append(builtin, "BINFMT_SCRIPT", "BINFMT_MISC")
		module = append(module, "TEST_ASYNC_DRIVER_PROBE", "NFS_FS")
	} else {
		// Assists heap memory attacks; best to keep interface disabled.
		missing = append(missing, "INET_DIAG")
	}

	if ver.isOrLater(3, 18) {
		builtin = append(builtin, "SND_PROC_FS", "USB_CONFIGFS_F_FS", "ESD_FS")
		module = append(module, "USB_F_FS")
		enabled = append(enabled, "CONFIGFS_FS")
		// Like FW_LOADER_USER_HELPER, these may be exploited by userspace.
		// We run udev everywhere which uses netlink sockets for event
		// propagation rather than executing programs, so don't need this.
		missing = append(missing, "UEVENT_HELPER", "UEVENT_HELPER_PATH")
	}

	if ver.isOrLater(4, 4) {
		// Security; make sure usermode helper is our tool for linux-4.4+.
		builtin = append(builtin, "STATIC_USERMODEHELPER")
		value["STATIC_USERMODEHELPER_PATH"] = `"/sbin/usermode-helper"`
	} else {
		// For kernels older than linux-4.4.
		// TODO(crbug/976562): In the original test it was actually non-op (probably a bug). Check this doesn't cause error and remove this comment.
		builtin = append(builtin, "EXT4_USE_FOR_EXT23")
	}

	// Security; marks data segments as RO/NX, text as RO.
	if ver.isOrLater(4, 11) {
		builtin = append(builtin, "STRICT_KERNEL_RWX", "STRICT_MODULE_RWX")
	} else {
		builtin = append(builtin, "DEBUG_RODATA", "DEBUG_SET_MODULE_RONX")
	}
	if arch == "aarch64" {
		builtin = append(builtin, "DEBUG_ALIGN_RODATA")
	}

	if ver.isOrLater(4, 19) {
		builtin = append(builtin, "HAVE_EBPF_JIT", "BPF_JIT_ALWAYS_ON", "STACKPROTECTOR")
	} else {
		// Security; adds stack buffer overflow protections.
		builtin = append(builtin, "CC_STACKPROTECTOR")
		// bpf(2) syscall can be used to generate code patterns in kernel memory.
		missing = append(missing, "BPF_SYSCALL")
	}

	isX86Family := regexp.MustCompile(`^i\d86$`).MatchString(arch) || arch == "x86_64"
	if isX86Family {
		// Kernel: make sure port 0xED is the one used for I/O delay.
		builtin = append(builtin, "IO_DELAY_0XED")
		same = append(same, [2]string{"IO_DELAY_TYPE_0XED", "DEFAULT_IO_DELAY_TYPE"})

		// Security; make sure NX page table bits are usable.
		if arch == "x86_64" {
			builtin = append(builtin, "X86_64")
		} else {
			builtin = append(builtin, "X86_PAE")
		}
	}

	return &kernelConfigCheck{
		exclusive: exclusive,
		builtin:   builtin,
		module:    module,
		enabled:   enabled,
		value:     value,
		same:      same,
		missing:   missing,
	}
}

func (c *kernelConfigCheck) test(conf map[string]string, s *testing.State) {
	for _, k := range c.builtin {
		if got := conf[k]; got != "y" {
			s.Errorf("%s: got %s, want y", k, got)
		}
	}
	for _, k := range c.module {
		if got := conf[k]; got != "m" {
			s.Errorf("%s: got %s, want m", k, got)
		}
	}
	for _, k := range c.enabled {
		if got := conf[k]; got != "y" && got != "m" {
			s.Errorf("%s: got %s, want y or m", k, got)
		}
	}
	for k, want := range c.value {
		if got := conf[k]; got != want {
			s.Errorf("%s: got %s, want %v", k, got, want)
		}
	}
	for _, k := range c.same {
		if x, y := conf[k[0]], conf[k[1]]; x != y {
			s.Errorf("Values of %s and %s should be the same but were %s and %s", k[0], k[1], x, y)
		} else if x == "" {
			s.Errorf("%s and %s should exist but didn't", k[0], k[1])
		}
	}
	for _, k := range c.missing {
		if got, ok := conf[k]; ok {
			s.Errorf("%s should not exist but was %s", k, got)
		}
	}

	// Test exclusive.
	declared := make(map[string]bool)
	for _, l := range [][]string{c.builtin, c.module, c.enabled} {
		for _, k := range l {
			declared[k] = true
		}
	}
	for k := range c.value {
		declared[k] = true
	}
	for _, k := range c.same {
		declared[k[0]] = true
		declared[k[1]] = true
	}
	for _, r := range c.exclusive {
		for k := range conf {
			if r.MatchString(k) && !declared[k] {
				s.Errorf("%s should be declared but wasn't", k)
			}
		}
	}
}
