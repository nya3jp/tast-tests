# Building a kernel and rootfs for virtio-fs

## Build a rootfs

The steps to build a root file system for the VM are a miniature version of the
steps used to build Chrome OS rootfs images: we will first build all the
packages we need (including their build-time dependencies) in a staging
directory and then copy just the built packages and their runtime dependencies
into the target file system.

To build a rootfs:

1. Get a vanilla gentoo system (the Chrome OS SDK will not work for this). You
   can create a gentoo chroot on your desktop by following the [installation
   instructions](https://wiki.gentoo.org/wiki/Handbook:AMD64/Installation/Stage)
   (start directly from the stage 3 tarball).
   
2. Edit your `/etc/portage/make.conf` file and add:

```shell
FEATURES="buildpkg"  # this is important for later
MAKEOPTS="-j100"  # or something appropriate for your machine
EMERGE_DEFAULT_OPTS="--usepkg"
```

3. Install `crossdev` and set up the cross-compiler (replace `$CHOST` with
   either `x86_64-linux-musl` or `aarch64-linux-musl`):

```shell
$ emerge -j --ask sys-devel/crossdev
$ crossdev --stable -t $CHOST
```

The first time you run `crossdev` it may complain about a missing overlay.  In
that case, follow the instructions in the [crossdev
section](https://wiki.gentoo.org/wiki/Custom_repository#Crossdev) of the Gentoo
wiki article on custom repositories. The general steps are:

```shell
$ mkdir -p /usr/local/portage-crossdev/{profiles,metadata}
$ echo 'crossdev' > /usr/local/portage-crossdev/profiles/repo_name
$ echo 'masters = gentoo' > /usr/local/portage-crossdev/metadata/layout.conf
$ chown -R portage:portage /usr/local/portage-crossdev
```

And then create `/etc/portage/repos.conf/crossdev.conf` with the following
contents: 

```shell
[crossdev]
location = /usr/local/portage-crossdev
priority = 10
masters = gentoo
auto-sync = no
```

4. If you're building on an x86_64 machine, fix the shared library symlink:

```shell
$ cd /usr/$CHOST/usr && ln -s lib lib64
```

5. Edit the `/usr/$CHOST/etc/portage/make.conf` file:

```shell
USE="-* ${ARCH}" # The '-*' turns off all extra options
PYTHON_TARGETS="python3_6"  # Needed because of the '-*' above
MAKEOPTS="-j100"  # or something appropriate for your machine
EMERGE_DEFAULT_OPTS="--usepkg"
```

6. Mark gcc and musl as provided packages so that we don't waste time
   rebuilding them.  Get the version numbers with `emerge --search
   cross-$CHOST/$PN`, where `$PN` is either `gcc` or `musl`.  Add the following
   to `/usr/$CHOST/etc/portage/profile/package.provided`:
   
```shell
sys-devel/gcc-$PV
sys-libs/musl-$PV
```

7. Set the profile. Replace `$ARCH` in the command below with either `amd64` for
   x86_64 systems and `arm64` for aarch64 systems.

```shell
$ ARCH=$ARCH PORTAGE_CONFIGROOT=/usr/$CHOST eselect profile set default/linux/$ARCH/17.0/musl
```

8. Remove the `perl-cleaner` dependency from the `perl` package. This is used
   for cleaning up old installations of perl but we don't have any since we're
   creating this chroot from scratch.  It also ends up transitively pulling in a
   bunch of extra dependencies that we don't need (like portage itself).

```shell
$ sed -i '/perl-cleaner/d' /var/db/repos/gentoo/dev-lang/perl/*.ebuild
$ find /var/db/repos/gentoo/dev-lang/perl/ -name '*.ebuild' -exec ebuild {} manifest \;
```

9. Create the `/var/db/repos/app-benchmarks/pjdfstest` directory and a
   `pjdfstest-20190822.ebuild` file inside it with the following contents:

```shell
# Copyright 2019 The Chromium OS Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=7

inherit autotools toolchain-funcs

GIT_HASH="ee51841e0d99d25ab18027770f6b6f0596a07574"
DESCRIPTION="A file system test suite that exercises POSIX system calls"
HOMEPAGE="https://github.com/pjd/pjdfstest"
SRC_URI="https://github.com/pjd/pjdfstest/archive/${GIT_HASH}.tar.gz -> ${P}.tar.gz"

LICENSE="BSD"
SLOT="0"
KEYWORDS="amd64 arm64"
IUSE=""

PATCHES=( "${FILESDIR}/${P}-fix-2038.patch" )
RDEPEND="
        dev-lang/perl
        dev-libs/openssl
        sys-apps/coreutils
        sys-apps/grep
        sys-apps/sed
        sys-apps/util-linux
        virtual/awk
        virtual/perl-Test-Harness
"

S="${WORKDIR}/${PN}-${GIT_HASH}"

src_configure() {
        tc-export CC
        eautoreconf
        econf
}

src_install() {
        exeinto /opt/pjdfstest/
        doexe pjdfstest

        insinto /opt/pjdfstest/
        doins -r tests
}
```

Add the `files/pjdfstest-20190822-fix-2038.patch` file with the following
contents:

```diff
diff --git a/tests/utimensat/09.t b/tests/utimensat/09.t
index ec7acbe..a3500b9 100644
--- a/tests/utimensat/09.t
+++ b/tests/utimensat/09.t
@@ -25,7 +25,9 @@ cd ${n1}

 create_file regular ${n0}
 expect 0 open . O_RDONLY : utimensat 0 ${n0} $DATE1 0 $DATE2 0 0
+todo Linux "Fix 2038 problem before 2038"
 expect $DATE1 lstat ${n0} atime
+todo Linux "Fix 2038 problem before 2038"
 expect $DATE2 lstat ${n0} mtime
```

We need the patch because the server runs as a 32-bit binary on arm devices and
there is no way for it to pass a 64-bit `time_t` value to the kernel. Now reate
the manifest and then emerge the packages we'll be using for the test:

```shell
$ ebuild /var/db/repos/gentoo/app-benchmarks/pjdfstest/pjdfstest-20190822.ebuild manifest
$ ${CHOST}-emerge -j -uva pjdfstest bash baselayout
```

10. Set up the target rootfs and install the packages we need into it:

```shell
$ mkdir /usr/${CHOST}-rootfs
$ PORTAGE_CONFIGROOT=/usr/${CHOST} emerge -j --usepkgonly --with-bdeps=n \
      --root-deps=rdeps --root=/usr/${CHOST}-rootfs --ask \
      pjdfstest bash baselayout
```

11. Create `/usr/${CHOST}-rootfs/bin/run-pjdfstest.sh` with the following contents:

```shell
#!/bin/bash

export PATH="/usr/bin:/usr/sbin:/bin:/sbin"

die() {
        echo "$@"
        exit 1
}

mount -t proc proc /proc || die "proc"
mount -t sysfs sys /sys  || die "sysfs"
mount -t tmpfs tmp /tmp || die "tmp"
mount -t tmpfs run /run || die "run"

mkdir -p /var/tmp/pjdfstest || die "mkdir"
cd /var/tmp/pjdfstest

prove -rv /opt/pjdfstest/tests
if [ "$?" -ne 0 ]; then
        echo FAIL
else
        echo PASS
fi
```

This script will run as init in the VM.  Don't forget to mark it executable:

```shell
$ chmod +x /usr/${CHOST}-rootfs/bin/run-pjdfstest.sh
```

12. Unpack the libc we built earlier into the target rootfs (this is why we set
    `FEATURES="buildpkg"` in step 2):

```shell
$ mkdir /tmp/musl
$ tar xaf /var/cache/binpkgs/cross-${CHOST}/musl-${PV}.tbz2 -C /tmp/musl
$ rsync --archive /tmp/musl/usr/${CHOST}/ /usr/${CHOST}-rootfs/
$ rm -r /tmp/musl
```

Make sure to use the trailing `/` in the `rsync` command to sync the *contents*
of the directory rather than the directory itself.

> If you forgot to set `FEATURES="buildpkg"` in step 2 then you should be able
> to re-create the toolchain packages with:
> 
> ```shell
> $ quickpkg --include-unmodified-config=y cross-${ARCH}/gcc
> $ quickpkg --include-unmodified-config=y cross-${ARCH}/musl
> $ quickpkg --include-unmodified-config=y cross-${ARCH}/binutils
> $ quickpkg --include-unmodified-config=y cross-${ARCH}/linux-headers
> ```

Now fix up a couple of the symlinks:

```shell
$ cd /usr/${CHOST}-rootfs/lib/
$ rm ld-musl-${ARCH}.so.1
$ ln -s ../usr/lib/libc.so ld-musl-${ARCH}.so.1
$ cd /usr/${CHOST}-rootfs/usr/bin
$ rm ldd
$ ln -s ../../lib/ld-musl-${ARCH}.so.1 ldd
```

13. Create some mount points on the rootfs:

```shell
$ mkdir -p /usr/${CHOST}-rootfs/{proc,dev,sys,tmp,run}
```

14. Package the rootfs in a tarball:

```shell
$ tar cJf rootfs-${ARCH}.tar.xz -C /usr/${CHOST}-rootfs/ .
```

Don't forget the trailing dot in the command.

## Build a kernel

Get the linux sources for version 5.4 or newer.  Copy the crostini kernel config
from the 4.19 kernel tree.  For x86\_64:

```shell
$ cp ~/chromiumos/src/third_party/kernel/v4.19/arch/x86/configs/chromiumos-container-vm-x86_64_defconfig .config
```

And for aarch64:

```shell
$ cp ~/chromiumos/src/third_party/kernel/v4.19/arch/arm64/configs/chromiumos-container-vm-arm64_defconfig .config
```

Enable `CONFIG_VIRTIO_FS` (for x86\_64 you'll also need to enable `CONFIG_PCI`
and `CONFIG_VIRTIO_PCI`) and build the kernel (replace `${ARCH}` with either
x86\_64 or arm64):

```shell
$ export ARCH=${ARCH}
$ export CROSS_COMPILE=${CHOST}-
$ make olddefconfig
$ make menuconfig # Enable CONFIG_VIRTIO_FS here
$ make -j100
```

For x86, you'll need to strip the resulting kernel:

```shell
$ strip --strip-debug -o vmlinux.stripped vmlinux
```

For arm64, we can use `arch/arm64/boot/Image` directly.

## Test the kernel and rootfs

Copy the kernel and rootfs onto the device and unpack the rootfs tarball.  Then
run (on the device):

```shell
$ crosvm run \
      -p "root=/dev/root rootfstype=virtiofs init=/bin/run-pjdfstest.sh" \
      -s /run \
      --serial type=stdout,num=1,console=true,stdin=true \
      --shared-dir <path_to_rootfs>:/dev/root:type=fs \
      --disable-sandbox \
      <path_to_vmlinux>
```

The `--disable-sandbox` flag is needed because the test uses many `mknod` calls
and these will only succeed if the file system has `CAP_SYS_ADMIN` in the
initial namespace.
