Building a kernel and rootfs for virtio-fs
==========================================

# Build a rootfs

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

6. Mark gcc and glibc as provided packages so that we don't waste time
   rebuilding them.  Get the version numbers with `emerge --search
   cross-$CHOST/$PN`, where `$PN` is either `gcc` or `glibc`.  Add the following
   to `/usr/$CHOST/etc/portage/profile/package.provided`:
   
```shell
sys-devel/gcc-$PV
sys-libs/glibc-$PV
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

9. Add the `pjdfstest` package and build it along with `bash` and `baselayout`:

```shell
$ mkdir -p /var/db/repos/gentoo/app-benchmarks/pjdfstest
$ cat <<EOF > /var/db/repos/gentoo/app-benchmarks/pjdfstest/pjdfstest-20190822.ebuild
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
EOF
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

11. Create the script that will run as init in the VM:

```shell
$ cat <<EOF > /usr/${CHOST}-rootfs/bin/run-pjdfstest.sh
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
EOF
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

Make sure to use the trailing `/` in the `rsync` command to sync the *contents* of
the directory rather than the directory itself.

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

# Build a kernel

# Test the kernel and rootfs

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
