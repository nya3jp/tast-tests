# Hostap Hwsim Tests

The [hostap project] consists of `wpa_supplicant` and `hostapd`, used for WiFi
clients and APs respectively. The project includes a number of tests, many of
which are run under a [hwsim-based test framework]. They rely on the
`mac80211_hwsim` kernel module, which provides support for simulated client and
AP devices. See the upstream project documentation for more info.

The upstream test framework is integrated within Chrome OS to run via the
[wifi.HostapHwsim] Tast test wrapper. This test has some limited software
dependencies, which are installed via the `USE=wifi_hostap_test` feature flag.
Currently, this `USE` flag is enabled only on `BOARD=amd64-generic` and those
that inherit from it. The easiest way to use the tests is via a VM target, like
`BOARD=betty` (Google-only).

See these [slides] (Google-only) for a rough primer on how and why they were
integrated into Chrome OS.

## HOWTO

Below is a walkthrough on building and running a VM test, and modifying the
tests yourself. See the [cros_vm docs] for more thorough VM instructions.

```bash
# Build a board for a VM.
export BOARD=betty
./build_packages --board=${BOARD}
./build_image --board=${BOARD} test

# Start the VM.
cros_vm --start --board=${BOARD}

# Run the validity test.
tast -verbose run localhost:9222 wifi.HostapHwsim.validity

# Make modifications.
## Find the appropriate src/third_party/wpa_supplicant-*/
## source tree. See the hostap-test ebuild for its CROS_WORKON_LOCALNAME:
##   https://chromium.googlesource.com/chromiumos/overlays/chromiumos-overlay/+/refs/heads/main/net-wireless/hostap-test/hostap-test-9999.ebuild
## That is, src/third_party/wpa_supplicant-2.8 as of May 2020.
## Make your modifications to, e.g., tests/hwsim/test_<relevant_module>.py, or
## wpa_supplicant/<foo>.c.

# Deploy modifications.
cros_workon-${BOARD} start hostap-test
emerge-${BOARD} hostap-test && cros deploy --root=/usr/local localhost:9222 hostap-test

# Rerun validity tests.
tast -verbose run localhost:9222 wifi.HostapHwsim.validity

# Run a specific module, the 'ap_roam' module.
tast -verbose run -var=wifi.HostapHwsim.runArgs='-f ap_roam' \
    localhost:9222 wifi.HostapHwsim.full
```

## Tips

*   **Pitfall**: Tests are packaged via the `net-wireless/hostap-test` package,
    not the `net-wireless/wpa_supplicant` or `net-wireless/hostapd` packages.
    Be sure to `cros_workon` and `cros deploy` the correct one.
*   **Pitfall**: Until `cros deploy` learns to [deploy to the correct root],
    remember to use `--root=/usr/local` with `cros deploy`. Test-image-only
    packages are installed at `/usr/local`, so you should deploy the
    `hostap-test` package there too.
*   Logs are stored to the Tast results directory
    (`/tmp/tast/results/latest/tests/wifi.HostapHwsim.*/`). These include
    kernel logs and logs for one or more `hostapd` or `wpa_supplicant`
    instance, as well as packet captures.
*   You can pass arbitrary arguments to the `wifi.HostapHwsim.full` variant
    via the `-var=wifi.HostapHwsim.runArgs='...'` parameter, to run specific
    tests, or to add extra debugging information. e.g., the `-T` parameter
    captures kernel tracing information via `trace-cmd`. See the `--help`
descriptions for more info:

```
localhost ~ # /usr/local/libexec/hostap/tests/hwsim/run-all.sh -h
/usr/local/libexec/hostap/tests/hwsim/run-all.sh [-v | --valgrind | valgrind] [-t | --trace | trace]
	[-n <num> | --channels <num>] [-B | --build]
	[-c | --codecov ] [run-tests.py parameters]
localhost ~ # /usr/local/libexec/hostap/tests/hwsim/run-tests.py -h
usage: run-tests.py [-h] [--logdir <directory>] [-d | -q] [-S <sqlite3 db>]
                    [--prefill-tests] [--commit <commit id>] [-b <build>] [-L]
                    [-T] [-D] [--dbus] [--shuffle-tests] [--split SPLIT]
                    [--no-reset] [--long]
                    [-f <test module> [<test module> ...]] [-l <modules file>]
                    [-i]
                    [<test> [<test> ...]]

hwsim test runner

positional arguments:
  <test>                tests to run (only valid without -f)

optional arguments:
  -h, --help            show this help message and exit
  --logdir <directory>  log output directory for all other options, must be
                        given if other log options are used
  -d                    verbose debug output
  -q                    be quiet
  -S <sqlite3 db>       database to write results to
  --prefill-tests       prefill test database with NOTRUN before all tests
  --commit <commit id>  commit ID, only for database
  -b <build>            build ID
  -L                    List tests (and update descriptions in DB)
  -T                    collect tracing per test case (in log directory)
  -D                    collect dmesg per test case (in log directory)
  --dbus                collect dbus per test case (in log directory)
  --shuffle-tests       Shuffle test cases to randomize order
  --split SPLIT         split tests for parallel execution (<server
                        number>/<total servers>)
  --no-reset            Do not reset devices at the end of the test
  --long                Include test cases that take long time
  -f <test module> [<test module> ...]
                        execute only tests from these test modules
  -l <modules file>     test modules file name
  -i                    stdin-controlled test case execution
```

[hostap project]: https://w1.fi/
[hwsim-based test framework]: https://w1.fi/cgit/hostap/plain/tests/hwsim/README
[wifi.HostapHwsim]: hostap_hwsim.go
[slides]: https://goto.google.com/hostap-hwsim-slides
[cros_vm docs]: https://chromium.googlesource.com/chromiumos/docs/+/main/cros_vm.md
[deploy to the correct root]: https://crbug.com/341708
