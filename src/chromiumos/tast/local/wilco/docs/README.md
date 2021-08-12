# Setup guide for Wilco devices Tast tests

[TOC]

## Introduction

For years Chrome OS has been seen running on `ChromeOS EC` (cros_ec) developed by Google but there is another variant of the embedded controller `Dell EC`  that are recently being productionized in `sarien` and `drallion` boards manufactured by Dell. The EC is responsible for keyboard control, power sequencing, battery state, boot verification, thermal control, and related functionality and many more.

Most of the wilco tests require some specific device setup on which it successfully gets executed but the test files in [`Tast`] written in Go sometimes lack that kind of setup information. This doc acts as a single source of truth for the existing wilco tests or the tests that are going to be added in future to aid the developer to replicate the exact same setup on which it was tested by the author.

[`Tast`]: https://chromium.googlesource.com/chromiumos/platform/tast-tests/

## A bit of Servo

Servo is a very crucial component for writing some tests in Chrome OS even for wilco devices. It is capable of performing a lot of powerful things including turning on/off the DUT, controlling power delivery to DUT, key pressing, closing/opening the lid etc. There are documentations on different types of servos but make sure you understand how servo works fundamentally, the exposed commands by different servo versions and choose what kind of connection (remote proxy or direct connection) you do require for writing tests. Please see the reference for useful links.

There are different versions of servo (v2, v3, v4) and we are interested in [`ServoV4`], the most recent one, and [`Servo-Micro`] (uServo). ServoV4 comes in two variants: Type-A and Type-C. Type-C can act as a power delivery charger and charge the DUT if the servo board itself is connected to a power supply and has the `servo_pd_role` set to `src` while Type-A can be used to detect power shared through the DUT USB interface. Also, there is another variant of servo known as Servo-Micro which is connected to the DUT motherboard debug header to perform direct hardware-level stuff. By default, ServoV4 doesn’t have any hardware debug features on its own and for hardware-level debugging it must be paired with a uServo. However, recent Chrome OS devices support Closed Case Debug ([`CCD`]) feature where without opening the DUT physically to connect a uServo to the debug header, a SuzyQ cable gets connected to the DUT USB-C interface. It basically tells Cr50 OS, an embedded OS of a secure microcontroller in DUT, to go into debug mode.

## Test Setup Configurations

This section deals with the setup that ideally should be replicated for running wilco tests successfully. Add configuration details in this markdown and put a reference of that subsection to the respective test file.

### Servo Type-A with Servo Micro : Measuring USB VBUS output with DUT on-off state

This configuration requires a combination of ServoV4 Type-A and Servo-Micro. Both cables should be connected to DUT. Type-A should be connected to the DUT USB-A port that has a lightning bolt or a battery icon engraved into it. Type-A is responsible for detecting `vbus_power` coming through the USB VBUS interface while uServo aka servo micro controls the on/off state of DUT.

Used in:

- [`policy.DeviceUSBPowershare`]

### Servo Type-C with Servo Micro : Controlling power delivery with DUT on-off state

This configuration requires a combination of ServoV4 Type-C and Servo-Micro and both cables should be connected to DUT. Type-C which is connected to the DUT USB-C port acts as a power charger through its power delivery (PD) ability via flipping `servo_pd_role` from `src` to `snk` or vice versa. As usual, the uServo is responsible for managing DUT on-off state.

Used in:

- [`policy.DeviceBootOnAC`]

### Plain Servo Type-C: Controlling power delivery

This configuration requires a simple ServoV4 Type-C connected to a USB Type-C interface to control the power delivery to DUT.

- [`policy.DeviceBatteryChargeMode`]
- [`policy.DeviceAdvancedBatteryChargeMode`]
- [`policy.DevicePowerPeakShift`]

[`policy.DeviceUSBPowershare`]: ../../../remote/bundles/cros/policy/device_usb_powershare.go
[`policy.DeviceBootOnAC`]: ../../../remote/bundles/cros/policy/device_boot_on_ac.go
[`policy.DevicePowerPeakShift`]: ../../bundles/cros/policy/device_power_peak_shift.go
[`policy.DeviceBatteryChargeMode`]: ../../bundles/cros/policy/device_battery_charge_mode.go
[`policy.DeviceAdvancedBatteryChargeMode`]: ../../bundles/cros/policy/device_advanced_battery_charge_mode.go

## References

- [`ServoV4`], [`ServoV4.1`]
- [`Servo-Micro`]
- [`CCD`] - Closed Case Debug
- [`Servo Setup Diagrams`]: Though this doc is meant for firmware testing hardware setup, it possesses some cool graphics and elaborate details.

[`ServoV4`]: https://chromium.googlesource.com/chromiumos/third_party/hdctools/+/HEAD/docs/servo_v4.md
[`ServoV4.1`]: https://chromium.googlesource.com/chromiumos/third_party/hdctools/+/HEAD/docs/servo_v4p1.md
[`Servo-Micro`]: https://chromium.googlesource.com/chromiumos/third_party/hdctools/+/HEAD/docs/servo_micro.md
[`CCD`]: https://chromium.googlesource.com/chromiumos/third_party/hdctools/+/HEAD/docs/ccd.md
[`Servo Setup Diagrams`]: https://chromium.googlesource.com/chromiumos/third_party/autotest/+/HEAD/docs/faft-how-to-run-doc.md#hardware-setup
