## How to run autoupdate tests

So far the tests can only be run on leased devices from the test lab.
Note that TLW service instructions are subject to change. Please update the
documentation if the instructions don't work anymore.

### Install the TLW service

```bash
mkdir chrome_infra
cd chrome_infra
fetch infra   # or fetch infra_internal if you are a Googler
eval $(infra/go/env.py)
```

Add `$HOME/chrome_infra/infra/go` to your `$GOPATH` for example by appending
the following to `~/.profile`.

```
export GOPATH="$GOPATH:$HOME/chrome_infra/infra/go"
```

### Start the TLW service

```bash
cd ~/chrome_infra/infra/go/src/infra/cros/cmd
(cd prototype-tlw && go run . -port 7151) &
```

> **_TIP:_** Make sure the service is running using `ps aux | grep tlw`

If you get a `$GOPATH` error when starting the service, make sure you have
followed
[Tast Quickstart](https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/quickstart.md).

> **_TL;DR;_** Your `~/.profile` or `~/.bashrc` should contain:

```
export CHROMEOS_SRC=$HOME/chromiumos
# Main GOPATH, where extra binaries are installed.
export GOPATH=$HOME/go
# Append Tast repos to GOPATH
export GOPATH=${GOPATH}:${CHROMEOS_SRC}/src/platform/tast-tests:${CHROMEOS_SRC}/src/platform/tast
# Append Tast dependencies
export GOPATH=${GOPATH}:${CHROMEOS_SRC}/chroot/usr/lib/gopath
# chrome infra
export GOPATH=$GOPATH:$HOME/chrome_infra/infra/go
```

### Create ssh tunnel and run the test

Make sure that you can access the device without a password, for example by
using ssh keys.

Create ssh tunnel to access DUT from inside chroot:

```bash
autossh -L 2200:localhost:22 root@<dut>
```

Run the test inside chroot as follows:

```
(chroot) tast run -var=updateutil.tlwAddress="localhost:7151" localhost:2200 <test_name>
```
