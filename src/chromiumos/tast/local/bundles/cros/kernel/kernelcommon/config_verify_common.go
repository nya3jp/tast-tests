// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kernelcommon contains utilities for package kernel.
package kernelcommon

import (
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// ReadKernelConfig reads the kernel config key value pairs trimming CONFIG_ prefix from the keys.
func ReadKernelConfig(ctx context.Context) (map[string]string, error) {
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

// kernelConfigCheck contains configs to check.
type kernelConfigCheck struct {
	// Exclusive contains regexes. The kernel config keys matching a regex should be listed in one of the following fields except missing. The keys are compared with removing the CONFIG_ prefix.
	Exclusive []*regexp.Regexp
	// Builtin contains keys that should be set to y.
	Builtin []string
	// Module contains keys that should be set to m.
	Module []string
	// Enabled contains keys that should be set to y or m.
	Enabled []string
	// Value contains key and value pairs that should be set.
	Value map[string]string
	// Same contains pairs of keys that should be set to the same value.
	Same [][2]string
	// Optional contains keys that may or may not exist.
	Optional []string
	// Missing contains keys that shouldn't exist.
	Missing []string
}

// NewKernelConfigCheck creates a common kernelConfigCheck struct.
func NewKernelConfigCheck(ver *sysutil.KernelVersion, arch string) *kernelConfigCheck {
	Exclusive := []*regexp.Regexp{
		// Security; no surprise binary formats.
		regexp.MustCompile(`^BINFMT_`),
		// Security; no surprise filesystem formats.
		regexp.MustCompile(`^.*_FS$`),
		// Security; no surprise partition formats.
		regexp.MustCompile(`^.*_PARTITION$`),
	}
	Builtin := []string{
		// Validity checks; should be present in builds as builtins.
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

		// Binary formats.
		"BINFMT_ELF",

		// Filesystem formats.
		"DEBUG_FS",
		"ECRYPT_FS",
		"EXT4_FS",
		"PROC_FS",
		"SCSI_PROC_FS",

		// Partition formats.
		"EFI_PARTITION",
		// MAC is for external drive formatted on Macintosh.
		"MAC_PARTITION",
		"MSDOS_PARTITION",

		// Kernel hardening.
		// Settings that are commented out need to be enabled in the kernel first.
		// TODO(crbug.com/1061514): Start enabling these.

		// CONFIG_SHUFFLE_PAGE_ALLOCATOR=y (since v5.2)

		// CONFIG_INIT_ON_ALLOC_DEFAULT_ON=y (since v5.3)
	}
	Module := []string{
		// Validity checks; should be present in builds as modules.
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
	Enabled := []string{
		// Either module or enabled, depending on platform.
		"VIDEO_V4L2",
	}
	Value := map[string]string{
		// Security; NULL-address hole should be as large as possible.
		// Upstream kernel recommends 64k, which should be large enough
		// to catch nearly all dereferenced structures. For
		// compatibility with ARM binaries (even on x86) this needs to
		// be 32k.
		"DEFAULT_MMAP_MIN_ADDR": "32768",

		// Magic sysrq: we allow it on by default, but *only* for the CrOS addition sysrq-x
		// (dump and crash / SYSRQ_ENABLE_CROS_XKEY=0x1000).
		// The default should match the verified-mode choice we make in
		// src/platform2/init/cros_sysrq_init.cc.
		"MAGIC_SYSRQ_DEFAULT_ENABLE": "0x1000",
	}
	var Same [][2]string
	Optional := []string{
		// OVERLAY_FS is needed in moblab images, and allowed to exist in general. https://crbug.com/990741#c9
		"OVERLAY_FS",
		// EFIVAR_FS is needed in reven images.
		"EFIVAR_FS",
	}
	Missing := []string{
		// Never going to optimize to this CPU.
		"M386",
		// Config not in real kernel config var list.
		"CHARLIE_THE_UNICORN",
		// Dangerous; allows direct physical memory writing.
		"ACPI_CUSTOM_METHOD",
		// Dangerous; disables brk(2) ASLR.
		"COMPAT_BRK",
		// Dangerous; allows direct kernel memory writing.
		"DEVKMEM",
		// Dangerous; allows userspace to change kernel logging.
		"DYNAMIC_DEBUG",
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

		// Validity checks (binfmt); one disabled, one does not exist.
		"BINFMT_AOUT",
		"BINFMT_IMPOSSIBLE",

		// Validity checks (fs); ones disabled, one does not exist.
		"EXT2_FS",
		"EXT3_FS",
		"XFS_FS",
		"IMPOSSIBLE_FS",

		// Validity checks (partition); one disabled, one does not exist.
		"LDM_PARTITION",
		"IMPOSSIBLE_PARTITION",
	}

	if ver.IsOrLater(3, 10) {
		Builtin = append(Builtin, "BINFMT_SCRIPT")
	}

	if ver.IsOrLater(3, 14) {
		Builtin = append(Builtin, "BINFMT_SCRIPT", "BINFMT_MISC")
		Builtin = append(Builtin, "HARDENED_USERCOPY")
		Module = append(Module, "TEST_ASYNC_DRIVER_PROBE", "NFS_FS")
	} else {
		// Assists heap memory attacks; best to keep interface disabled.
		Missing = append(Missing, "INET_DIAG")
	}

	if ver.IsOrLater(3, 18) {
		Builtin = append(Builtin, "SND_PROC_FS", "USB_CONFIGFS_F_FS")
		Module = append(Module, "USB_F_FS")
		Enabled = append(Enabled, "CONFIGFS_FS")
		// Like FW_LOADER_USER_HELPER, these may be exploited by userspace.
		// We run udev everywhere which uses netlink sockets for event
		// propagation rather than executing programs, so don't need this.
		Missing = append(Missing, "UEVENT_HELPER", "UEVENT_HELPER_PATH")
	}

	if ver.IsOrLater(4, 4) {
		// Security; make sure usermode helper is our tool for linux-4.4+.
		Builtin = append(Builtin, "STATIC_USERMODEHELPER")
		Value["STATIC_USERMODEHELPER_PATH"] = `"/sbin/usermode-helper"`
		// Security; prevent overflows that can be checked at compile-time.
		Builtin = append(Builtin, "FORTIFY_SOURCE")
	} else {
		// For kernels older than linux-4.4.
		Builtin = append(Builtin, "EXT4_USE_FOR_EXT23")
	}

	if arch == "aarch64" && ver.IsOrLater(4, 10) {
		// Security; use software emulated Privileged Access Never (PAN).
		Builtin = append(Builtin, "ARM64_SW_TTBR0_PAN")
	}

	// Security; marks data segments as RO/NX, text as RO.
	if ver.IsOrLater(4, 11) {
		Builtin = append(Builtin, "STRICT_KERNEL_RWX", "STRICT_MODULE_RWX")
	} else {
		Builtin = append(Builtin, "DEBUG_RODATA", "DEBUG_SET_MODULE_RONX")
	}
	if arch == "aarch64" && ver.IsOrLess(5, 6) {
		Builtin = append(Builtin, "DEBUG_ALIGN_RODATA")
	}

	if ver.IsOrLater(4, 14) {
		// Security; harden the SLAB/SLUB allocators against common freelist exploit methods.
		Builtin = append(Builtin, "SLAB_FREELIST_RANDOM")
		Builtin = append(Builtin, "SLAB_FREELIST_HARDENED")
		// Security; initialize uninitialized local variables, variable fields, and padding.
		// (Clang only).
		if ver.IsOrLater(5, 4) {
			Builtin = append(Builtin, "INIT_STACK_ALL_ZERO")
		} else {
			Builtin = append(Builtin, "INIT_STACK_ALL")
		}
		if arch != "armv7l" {
			// Security; randomizes the virtual address at which the kernel image is loaded.
			Builtin = append(Builtin, "RANDOMIZE_BASE")
			// Security; virtually map the kernel stack to better defend against overflows.
			Builtin = append(Builtin, "VMAP_STACK")
		}
	}

	if arch == "aarch64" && ver.IsOrLater(4, 16) {
		// Security: unmaps kernel from page tables at EL0 (KPTI)
		Builtin = append(Builtin, "UNMAP_KERNEL_AT_EL0")
	}

	if ver.IsOrLater(4, 19) {
		Builtin = append(Builtin, "HAVE_EBPF_JIT", "BPF_JIT_ALWAYS_ON", "STACKPROTECTOR", "STACKPROTECTOR_STRONG")
	} else {
		// Security; adds stack buffer overflow protections.
		Builtin = append(Builtin, "CC_STACKPROTECTOR")
		// bpf(2) syscall can be used to generate code patterns in kernel memory.
		Missing = append(Missing, "BPF_SYSCALL")
	}

	if arch == "aarch64" && ver.IsOrLater(5, 0) {
		// Security: uses a different stack canary for each task.
		Builtin = append(Builtin, "STACKPROTECTOR_PER_TASK")
	}

	if arch == "aarch64" && ver.IsOrLater(5, 9) && ver.IsOrLess(5, 10) {
		// SET_FS is used to mark architectures that have set_fs(). arm64 has this on 5.9 and 5.10 only.
		Builtin = append(Builtin, "SET_FS")
	}

	isX86Family := regexp.MustCompile(`^i\d86$`).MatchString(arch) || arch == "x86_64"
	if isX86Family {
		// Kernel: make sure port 0xED is the one used for I/O delay.
		Builtin = append(Builtin, "IO_DELAY_0XED")
		if ver.IsOrLess(4, 19) {
			Same = append(Same, [2]string{"IO_DELAY_TYPE_0XED", "DEFAULT_IO_DELAY_TYPE"})
		}

		// Security; make sure NX page table bits are usable.
		if arch == "x86_64" {
			Builtin = append(Builtin, "X86_64")
		} else {
			Builtin = append(Builtin, "X86_PAE")
		}

		// Kernel hardening.
		Builtin = append(Builtin, "PAGE_TABLE_ISOLATION")
		// Builtin = append(Builtin, "RANDOMIZE_MEMORY")

		// Retpoline is a Spectre v2 mitigation.
		if ver.IsOrLater(3, 18) {
			Builtin = append(Builtin, "RETPOLINE")
		}
		// Dangerous; disables VDSO ASLR.
		Missing = append(Missing, "COMPAT_VDSO")
	}

	return &kernelConfigCheck{
		Exclusive: Exclusive,
		Builtin:   Builtin,
		Module:    Module,
		Enabled:   Enabled,
		Value:     Value,
		Same:      Same,
		Optional:  Optional,
		Missing:   Missing,
	}
}

func (c *kernelConfigCheck) Test(conf map[string]string, s *testing.State) {
	for _, k := range c.Builtin {
		if got := conf[k]; got != "y" {
			s.Errorf("%s: got %s, want y", k, got)
		}
	}
	for _, k := range c.Module {
		if got := conf[k]; got != "m" {
			s.Errorf("%s: got %s, want m", k, got)
		}
	}
	for _, k := range c.Enabled {
		if got := conf[k]; got != "y" && got != "m" {
			s.Errorf("%s: got %s, want y or m", k, got)
		}
	}
	for k, want := range c.Value {
		if got := conf[k]; got != want {
			s.Errorf("%s: got %s, want %v", k, got, want)
		}
	}
	for _, k := range c.Same {
		if x, y := conf[k[0]], conf[k[1]]; x != y {
			s.Errorf("Values of %s and %s should be the same but were %s and %s", k[0], k[1], x, y)
		} else if x == "" {
			s.Errorf("%s and %s should exist but didn't", k[0], k[1])
		}
	}
	for _, k := range c.Missing {
		if got, ok := conf[k]; ok {
			s.Errorf("%s should not exist but was %s", k, got)
		}
	}

	// Test Exclusive.
	declared := make(map[string]bool)
	for _, l := range [][]string{c.Builtin, c.Module, c.Enabled, c.Optional} {
		for _, k := range l {
			declared[k] = true
		}
	}
	for k := range c.Value {
		declared[k] = true
	}
	for _, k := range c.Same {
		declared[k[0]] = true
		declared[k[1]] = true
	}
	for _, r := range c.Exclusive {
		for k := range conf {
			if r.MatchString(k) && !declared[k] {
				// Construct error message.
				var allowed []string
				for d := range declared {
					if r.MatchString(d) {
						allowed = append(allowed, d)
					}
				}
				s.Errorf("Setting %q found in config via %q when only %q are allowed", k, r, allowed)
			}
		}
	}
}
