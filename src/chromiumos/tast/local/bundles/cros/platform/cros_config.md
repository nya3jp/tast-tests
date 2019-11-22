# platform.CrOSConfig Test HOWTO

The platform.CrosConfig Tast test is a safety net for validating changes to
identity, configuration or any CLI did not break the device you are working on
or other seemingly unrelated devices.

The test will read in a set of mosys and cros_config commands to execute on the
device under test (DUT) and check the results against a golden file containing
the expected output.

[TOC]

## Set Up a New Device

Setting up a new device involves two steps, setting up the device specific
commands and creating a new golden results file.

### Creating Device Specific Commands YAML File

Each device needs a `cros_config_device_commands.yaml` file to configure what
commands are run on the DUT. For a unibuild configuration this file will contain
all the commands for all configurations, but does not require all to be setup
at once.

The device specific commands file should inherit the
`cros_config_test_common.yaml` file to pick up all of the common commands, but
it isn't required.

During the build the common commands will be merged and a single commands JSON
file will be created and installed on dev and test images automatically.

A simple device specific commands YAML file for nautilus and nautiluslte looks
like the following:

```yaml
imports:
  - "cros_config_test_common.yaml"

chromeos:
  devices:
    - device-name: "nautilus"
      command-groups:
        - *mosys_base_cmds
        - *cros_config_unibuild_cmds
    - device-name: "nautiluslte"
      command-groups:
        - *mosys_base_cmds
        - *cros_config_unibuild_cmd
```

The real nautilus commands file is
[here](https://chrome-internal.googlesource.com/chromeos/overlays/overlay-nautilus-private/+/HEAD/chromeos-base/chromeos-config-bsp/tast_files/cros_config_device_commands.yaml)

If you only want to test the common commands this is all you need to set up. If
you want to extend the commands you will need to create a new custom command
group and add it to the command-groups list.

See the the [Command Groups](#cmd-grps) section for information on build
command groups.

### Unibuild Device Specific Command Location

For unibuilds, the device specific commands YAML lives in the private-overlays
repo under the `overlay-<board>-private/chromeos-base/chromeos-config-bsp` directory.

Create a new directory named `tast_files` and place the new YAML file in this
directory. This keeps the testing configuration close to the `model.yaml` configuration.

An example is the [Nautilus](https://chrome-internal.googlesource.com/chromeos/overlays/overlay-nautilus-private/+/HEAD/chromeos-base/chromeos-config-bsp/tast_files/cros_config_device_commands.yaml)
test configuration.

NOTE: The testing configuration needed a seperate directory because the unibuild
configuration picks up all YAML files in the `files` directory causing errors
during the build.

### Non-Unibuild Device Specific Command Location

For non-unibuilds, the device specific commands YAML also lives in the private
overlays repo under the
`overlay-<board>-private/chromeos-base/chromeos-bsp-<board>-private`
directory in a new directory named `tast_files`.

An example is the [Caroline](https://chromium.googlesource.com/chromeos/overlays/overlay-caroline-private/+/HEAD/chromeos-base/chromeos-bsp-caroline-private/tast_files/cros_config_device_commands.yaml)
test configuration.

### Update the ebuild

Updating the ebuild only involves two steps:

 1.  Inherit the `cros-config-test.eclass`.
 2.  In the `src_install()` step add the `install_cros_config_test_files`
     command.

Emerge the ebuild and it should copy the device specific commands YAML and any
golden results files to the `/build/<board>/tmp/chromeos-config/tast` directory.

## Modifying an Existing Device

To add or remove commands for an existing devices setup is simply editing the
device specific commands YAML and its corresponding golden file. Deploy to the
DUT and run the test. See the [Test Comparison and Errors](#errors) section on
what will cause test failures and what will pass.

If you add or remove common commands you should update all of the devices
golden files with the new results.

## Command Groups {#cmd-grps}

All commands run on the DUT need to be included in a YAML Command Group. A
Command Group should conform to the following schema:

```yaml
group-name: &group-anchor
  name: 'command to run'
    args:
      - 'command 1 args'
      - 'command 2 args'
```

For examples see [cros_config_test_common.yaml](https://chromium.googlesource.com/chromiumos/overlays/chromiumos-overlay/+/HEAD/chromeos-base/cros-config-test/files/cros_config_test_common.yaml)
The common commands YAML defines two command groups, one for the `mosys` command
and another for the `cros_config` command.

## Common Commands

The common commands are a set of Command Groups that can be added to a device
specific commands list. The common commands need to work on all devices.

The [cros_config_test_common.yaml](https://chromium.googlesource.com/chromiumos/overlays/chromiumos-overlay/+/HEAD/chromeos-base/cros-config-test/files/cros_config_test_common.yaml)
is located in the
`third_party/chromeos-overlay/chromeos-base/cros-config-test/files` directory.

When updating the common commands, you will need to update all of the golden
results files for all devices that include that command group. See the
[Creating and Updating Golden Results File](#golden) section.

## Merging the Commands and Final Packaging

To merge the common commands and device specific commands:

```
emerge-<board> chromeos-base/cros-config-test
```

This builds the final tarball with the `cros_config_test_commands.json` file
and all of the golden results files.

During the `build_image` stage for dev and test images this package will be
included in the final package. `cros-config-test` is included in the virtual
target `target-chromium-os-test`.

## Running and Checking Results

If you have a new setup or have made modifications to the commands or golden
files you will need to deploy the new package to the DUT.

```
cros deploy --board=<board> --root=/usr/local <DUT ipaddr> chromeos-base/cros-config-test
```

Run the Go Tast test:

```
tast run <DUT ipaddr> platform.CrosConfig
```

For more information on running the tests see the
[Tast running tests docs](https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/running_tests.md).

## Interpreting the Results

The test will output commands that it is running and at the bottom will give
an overall pass or failure for the platform.CrosConfig test. The results of the
test will be copied back to the host you ran the command from to the
`/tmp/tast/results/latest` directory.

The results directory will contain several logs and directories, the most
interesting are:

*   `system_logs` directory contains the contents of the system logs during
    the run.
*   `crashes` directory contains crash results from the run, empty if none.
*   `tests/platform.CrosConfig` directory contains `log.txt` of
    CrosConfig messages.
*   `tests/platform.CrosConfig` directory contains the new golden file named
    `<board>_output.json`.

See the following [Test Comparison and Errors](#errors) section on how errors
are determined and the
[Tast interpreting test results docs](https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/running_tests.md#Interpreting-test-results).

## Test Comparison and Errors {#errors}

To make life livable for everybody, the only failed comparisons between a test
run and an existing golden file are if and only if the exact command exists in
the golden file and the values differ.

See details below of the different scenarios:

*   **No commands JSON file** - This scenario reflects a device that is most
    likely not setup for cros_config testing. The device will quickly
    PASS the test as this scenario is defined as not an error.
*   **No device golden file** - This scenario most likely reflects a device
    that is in development. The commands JSON file exists and all commands
    will be run. The test will PASS and log a warning about the missing
    golden file.
*   **Commands missing from golden file** - This scenario will PASS the test
    and log a warning about the missing command and associated value. This
    will stop massive CQ failures if a new common command were added until
    all of the golden files can be updated.
*   **Extra commands in golden file** - This scenario will PASS the test and log
    a warning about the extra command and value in the golden file. As above
    this stop massive CQ failures if a common command is removed from the
    input.
*   **The set of commands in input and golden are equal, one or more output
    values differ** - This scenario is the only one that will trigger a test
    FAIL. All of the differing commands and their values will be logged as
    an error to the output.
*   **The set of commands in input and golden are equal, no differences in
    output values** - This is the happy path and will not log much info and
    test will PASS.

Crashes of any of the commands will automatically cause the test to FAIL.

## Creating and Updating Golden Results File {#golden}

To create or update a golden results file you can simply copy the
`/tmp/tast/results/latest/tests/platform.CrosConfig/<board>_output.json` output
file of a test run, if correct, to the device's `tast_files` directory. Build a
new cros-config-test package, re-deploy and re-run the test to validate the run
passes with no errors.

If you modify a common command group, you will need to update all of the devices
that include the common command group. There is currently no easy way to do this
except to run the new common config against all devices and add the new golden
results file to the device's `tast_files` directory as documented above.

## platform.CrosConfig Tast Test

The platform.CrosConfig test is a Go program that is part of the Tast cros
platform bundle of local tests. Local Tast tests are tests that are run on the
DUT. The [cros_config.go](https://chromium.googlesource.com/chromiumos/platform/tast-tests/+/HEAD/src/chromiumos/tast/local/bundles/cros/platform/cros_config.go)
test lives in the
`src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/platform`
directory. The executable will automatically be built and included in all
dev and test images.

You shouldn't need to modify this program for setting up and running the
platform.CrosConfig test.

We are in trying to get this test to be part of the normal CQ run, at this time
it is still flagged as 'informational' so it will be excluded on all official
CQ runs.
