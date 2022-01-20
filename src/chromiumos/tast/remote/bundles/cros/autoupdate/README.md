## How to run autoupdate tests

So far the tests can only be run on leased devices from the test lab.

Install the TLW service.
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

Start the TLW service.
```bash
cd ~/chrome_infra/infra/go/src/infra/cros/cmd
(cd prototype-tlw && go run . -port 7151) &
```

Make sure that you can access the device without a password, for example by
using ssh keys.

Create ssh tunnel to access DUT from inside chroot.
```bash
autossh -L 2200:localhost:22 root@<dut>
```

Run the test inside chroot.
```
(chroot) tast run -var=updateutil.tlwAddress="localhost:7151" localhost:2200 <test_name>
```
