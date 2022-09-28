// Copyright 2019 The ChromiumOS Authors
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
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConfigVerify,
		Desc: "Examines a kernel build CONFIG list to make sure various things are present, missing, built as modules, etc",
		Contacts: []string{
			"jeffxu@chromium.org",
			"chromeos-kernel-test@google.com",
			"oka@chromium.org", // Tast port author
		},
		Attr: []string{"group:mainline"},
	})
}

// ConfigVerify reads the Linux kernel version and arch to verify validity of
// the information returned depending on version.
func ConfigVerify(ctx context.Context, s *testing.State) {
	ver, arch, err := sysutil.KernelVersionAndArch()
	if err != nil {
		s.Fatal("Failed to get kernel version and arch: ", err)
	}

	conf, err := readKernelConfig(ctx)
	if err != nil {
		s.Fatal("Failed to read kernel config: ", err)
	}

	newKernelConfigCheck(ver, arch).test(conf, s)
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
	same [][2]string
	// optional contains keys that may or may not exist.
	optional []string
	// missing contains keys that shouldn't exist.
	missing []string
}

func newKernelConfigCheck(ver *sysutil.KernelVersion, arch string) *kernelConfigCheck {
	exclusive := []*regexp.Regexp{
		// Security; no surprise binary formats.
		regexp.MustCompile(`^BINFMT_`),
		// Security; no surprise filesystem formats.
		regexp.MustCompile(`^.*_FS$`),
		// Security; no surprise partition formats.
		regexp.MustCompile(`^.*_PARTITION$`),
	}
	builtin := []string{
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
		// Security; make sure the ChromeOS LSM is in use.
		"SECURITY_CHROMIUMOS",

		// Binary formats.
		"BINFMT_ELF",
		"BINFMT_SCRIPT",
		"BINFMT_MISC",

		// Filesystem formats.
		"DEBUG_FS",
		"ECRYPT_FS",
		"EXT4_FS",
		"PROC_FS",
		"SCSI_PROC_FS",
		"SND_PROC_FS",
		"USB_CONFIGFS_F_FS",
		"ESD_FS",

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

		"HARDENED_USERCOPY",

		// Security; make sure usermode helper is our tool for linux-4.4+.
		"STATIC_USERMODEHELPER",

		// Security; prevent overflows that can be checked at compile-time.
		"FORTIFY_SOURCE",
	}
	module := []string{
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
		"NFS_FS",
		"USB_F_FS",

		"TEST_ASYNC_DRIVER_PROBE",
	}
	enabled := []string{
		// Either module or enabled, depending on platform.
		"VIDEO_V4L2",
		"CONFIGFS_FS",
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

		// Magic sysrq: we allow it on by default, but *only* for the CrOS addition sysrq-x
		// (dump and crash / SYSRQ_ENABLE_CROS_XKEY=0x1000).
		// The default should match the verified-mode choice we make in
		// src/platform2/init/cros_sysrq_init.cc.
		"MAGIC_SYSRQ_DEFAULT_ENABLE": "0x1000",

		// Security; make sure usermode helper is our tool for linux-4.4+.
		"STATIC_USERMODEHELPER_PATH": "/sbin/usermode-helper",
	}
	var same [][2]string
	optional := []string{
		// OVERLAY_FS is needed in moblab images, and allowed to exist in general. https://crbug.com/990741#c9
		"OVERLAY_FS",
		// EFIVAR_FS is needed in reven images.
		"EFIVAR_FS",
	}
	missing := []string{
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

		// Like FW_LOADER_USER_HELPER, these may be exploited by userspace.
		// We run udev everywhere which uses netlink sockets for event
		// propagation rather than executing programs, so don't need this.
		"UEVENT_HELPER",
		"UEVENT_HELPER_PATH",
	}

	if arch == "aarch64" && ver.IsOrLater(4, 10) {
		// Security; use software emulated Privileged Access Never (PAN).
		builtin = append(builtin, "ARM64_SW_TTBR0_PAN")
	}

	// Security; marks data segments as RO/NX, text as RO.
	if ver.IsOrLater(4, 11) {
		builtin = append(builtin, "STRICT_KERNEL_RWX", "STRICT_MODULE_RWX")
	} else {
		builtin = append(builtin, "DEBUG_RODATA", "DEBUG_SET_MODULE_RONX")
	}
	if arch == "aarch64" && ver.IsOrLess(5, 6) {
		builtin = append(builtin, "DEBUG_ALIGN_RODATA")
	}

	if ver.IsOrLater(4, 14) {
		// Security; harden the SLAB/SLUB allocators against common freelist exploit methods.
		builtin = append(builtin, "SLAB_FREELIST_RANDOM")
		builtin = append(builtin, "SLAB_FREELIST_HARDENED")
		// Security; initialize uninitialized local variables, variable fields, and padding.
		// (Clang only).
		if ver.IsOrLater(5, 4) {
			builtin = append(builtin, "INIT_STACK_ALL_ZERO")
		} else {
			builtin = append(builtin, "INIT_STACK_ALL")
		}
		if arch != "armv7l" {
			// Security; randomizes the virtual address at which the kernel image is loaded.
			builtin = append(builtin, "RANDOMIZE_BASE")
			// Security; virtually map the kernel stack to better defend against overflows.
			builtin = append(builtin, "VMAP_STACK")
		}
	}

	if arch == "aarch64" && ver.IsOrLater(4, 16) {
		// Security: unmaps kernel from page tables at EL0 (KPTI)
		builtin = append(builtin, "UNMAP_KERNEL_AT_EL0")
	}

	builtin += "STACKPROTECTOR"
	if ver.IsOrLater(4, 19) {
		builtin += "STACKPROTECTOR_STRONG"
	}

	// Needed for Spectre.
	if ver.IsOrLater(4, 19) {
		builtin += "HAVE_EBPF_JIT"
		builtin += "BPF_JIT_ALWAYS_ON"
	}

	// BPF_SYSCALL is in kernel 4.19 and 5.4 because BPF_JIT_ALWAYS_ON depends on it.
	// We don't check it for 4.19 and 5.4 because bpf syscall is blocked by LSM.
	// BPF_SYSCALL is not in kernels <= 4.14.
	if ver.IsOrLess(4, 14) {
		missing += "BPF_SYSCALL"
	}

	// BPF_SYSCALL is in kernels >= 5.10.
	if ver.IsOrLater(5, 10) {
		builtin += "BPF_SYSCALL"
	}

	if arch == "aarch64" && ver.IsOrLater(5, 0) {
		// Security: uses a different stack canary for each task.
		builtin = append(builtin, "STACKPROTECTOR_PER_TASK")
	}

	if arch == "aarch64" && ver.IsOrLater(5, 9) && ver.IsOrLess(5, 10) {
		// SET_FS is used to mark architectures that have set_fs(). arm64 has this on 5.9 and 5.10 only.
		builtin = append(builtin, "SET_FS")
	}

	isX86Family := regexp.MustCompile(`^i\d86$`).MatchString(arch) || arch == "x86_64"
	if isX86Family {
		// Kernel: make sure port 0xED is the one used for I/O delay.
		builtin = append(builtin, "IO_DELAY_0XED")
		if ver.IsOrLess(4, 19) {
			same = append(same, [2]string{"IO_DELAY_TYPE_0XED", "DEFAULT_IO_DELAY_TYPE"})
		}

		// Security; make sure NX page table bits are usable.
		if arch == "x86_64" {
			builtin = append(builtin, "X86_64")
		} else {
			builtin = append(builtin, "X86_PAE")
		}

		// Kernel hardening.
		builtin = append(builtin, "PAGE_TABLE_ISOLATION")
		// builtin = append(builtin, "RANDOMIZE_MEMORY")

		// Retpoline is a Spectre v2 mitigation.
		builtin = append(builtin, "RETPOLINE")
		// Dangerous; disables VDSO ASLR.
		missing = append(missing, "COMPAT_VDSO")
	}

	return &kernelConfigCheck{
		exclusive: exclusive,
		builtin:   builtin,
		module:    module,
		enabled:   enabled,
		value:     value,
		same:      same,
		optional:  optional,
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
	for _, l := range [][]string{c.builtin, c.module, c.enabled, c.optional} {
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
