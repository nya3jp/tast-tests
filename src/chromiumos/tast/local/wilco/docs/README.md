# Tast tests for Wilco devices with the required Setup Guide

[TOC]

## Introduction

For years Chromium OS has been seen running on `ChromeOS EC` (cros_ec) developed by Google but there is another variant of the embedded controller `Wilco EC`  that are recently being productionized in `sarien` and `drallion` boards manufactured by DELL. The EC is responsible for keyboard control, power sequencing, battery state, boot verification, thermal control, and related functionality and many more.

Most of the wilco tests require some specific device setup on which it successfully gets executed but the test files in [`Tast`] written in go lack that kind of information. Also, there is a very little/negligible amount of information floating around the web in context of wilco policy testing. So this doc acts as a single source of truth for the existing wilco tests or the tests that are going to be added in future to aid the developer to replicate the exact same setup on which it was tested by the author.

## A bit of Servo

Servo is a very crucial component for writing tests in chromeOS even for wilco devices. It is capable of performing a lot of powerful things that includes turning off the DUT, fiddling with DUT power delivery, performing different keypresses, closing/opening the lid etc. There are very good customized documentation on different types of servo but make sure you understand how servo works, the connection with DUT before proceeding to write tests. Please see the reference for useful links.

In layman’s terms, there are different versions of servo (v2, v3, v4) and we are interested in [`ServoV4`] the most recent one. ServoV4 comes in two variants: Type-A and Type-C. Type-C can act as a power delivery charger and potentially could charge the DUT if the servo board itself is connected to a power supply and has the `servo_pd_role` set to `src`. Also, there is another variant of servo known as [`Servo-Micro`] which is connected to the DUT motherboard debug header to perform direct hardware-level stuff. By default,  ServoV4 doesn’t have any hardware debug features on its own and for hardware-level debugging it must be paired with a Servo-Micro.

[`Tast`]: https://chromium.googlesource.com/chromiumos/platform/tast-tests/

## Device setup for Wilco tests

This section deals with the setup that ideally should be replicated for running wilco tests successfully. To augment this section please follow the below syntax:

```md
### Package.Test_Name | [Remote|Local]
- **Info**: Contains short information about the test. Any additional information must go here.
- **Setup**: The intended setup guide.
```

### policy.DeviceUSBPowershare | Remote

- **Info**: Verifies DeviceUsbPowerShareEnabled policy that enables sharing power through USB-A when DUT is in a power-off state.
- **Setup**: This test requires a combination of ServoV4 type A and Servo-Micro. Both cables should be connected to DUT. Type A should be connected to the DUT USB-A port that has a lightning bolt or a battery icon engraved into it.

### policy.DeviceBootOnAC | Remote

- **Info**: Verifies DeviceBootOnAcEnabled policy that boots the device from the off state by plugging in a power supply.
- **Setup**: This test requires a combination of ServoV4 type C and Servo-Micro. Both cables should be connected to DUT. Type C which is connected to the DUT USB-C port acts as a power charger through its power delivery (PD) ability.

### policy.DeviceBatteryChargeMode | Local

- **Info**: Verifies DeviceBatteryCharge policies, a group of power management policies, dynamically controls battery charging state to minimize stress and wear-out due to the exposure of rapid charging/discharging cycles and extends the battery life.
- **Setup**: Requires a ServoV4 Type C for controlling power delivery during testing.

### policy.DeviceAdvancedBatteryChargeMode | Local

- **Info**: Verifies DeviceAdvancedBatteryCharge policy group (power saving policy) that lets users maximize the battery health by using a standard charging algorithm and other techniques during non-working hours.
- **Setup**: Requires a ServoV4 Type C for controlling power delivery during testing.

### policy.DevicePowerPeakShift | Local

- **Info**: Verifies DevicePowerPeakShift policy group (power saving policy) that minimize alternating current usage during peak hours.
- **Setup**: Requires a ServoV4 Type C for controlling power delivery during testing.

## References

- [`ServoV4`], [`ServoV4.1`]
- [`Servo-Micro`]
- [`Servo Setup Diagrams`]: Though this doc is meant for firmware testing hardware setup, it possesses some cool graphics and elaborate details.

[`ServoV4`]: https://chromium.googlesource.com/chromiumos/third_party/hdctools/+/HEAD/docs/servo_v4.md
[`ServoV4.1`]: https://chromium.googlesource.com/chromiumos/third_party/hdctools/+/HEAD/docs/servo_v4p1.md
[`Servo-Micro`]: https://chromium.googlesource.com/chromiumos/third_party/hdctools/+/HEAD/docs/servo_micro.md
[`Servo Setup Diagrams`]: https://chromium.googlesource.com/chromiumos/third_party/autotest/+/HEAD/docs/faft-how-to-run-doc.md#hardware-setup
