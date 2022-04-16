# How to build a test OpenWrt image

## Install system dependencies
See https://openwrt.org/docs/guide-user/additional-software/imagebuilder#prerequisites for instructions.

## Build the custom image
1. Find your router in the [list of supported devices](https://openwrt.org/supported_devices) ([table lookup](https://openwrt.org/toh/start)).
2. Open the data page associated with your router from the supported devices table (e.g. https://openwrt.org/toh/hwdata/ubiquiti/ubiquiti_unifi_6_lite).
3. Navigate to the directory of where the "Firmware OpenWrt Upgrade URL" is located (e.g. from `https://downloads.openwrt.org/releases/21.02.2/targets/ramips/mt7621/openwrt-21.02.2-ramips-mt7621-ubnt_unifi-6-lite-squashfs-sysupgrade.bin` go to https://downloads.openwrt.org/releases/21.02.2/targets/ramips/mt7621).
4. Scroll down to the bottom of the page to the "Supplementary Files" section and copy the URL (you do not need to download it now) to the image builder archive, which should match `openwrt-imagebuilder-*.tar.xz` (e.g. https://downloads.openwrt.org/releases/21.02.2/targets/ramips/mt7621/openwrt-imagebuilder-21.02.2-ramips-mt7621.Linux-x86_64.tar.xz).
5. Run the build script with just the image builder URL to download and unpack the builder, as well as print its available build profiles:
Example:
```bash
$ bash ./build_openwrt_os_image.sh https://downloads.openwrt.org/releases/21.02.2/targets/ramips/mt7621/openwrt-imagebuilder-21.02.2-ramips-mt7621.Linux-x86_64.tar.xz
```
6. Choose the correct build profile based on your device. You can find it in your devices "Firmware OpenWrt Upgrade URL" as well (e.g. `ubnt_unifi-6-lite`)
7. Run the build script again with the same image builder URL and the build profile that you selected:
Example:
```bash
$ bash ./build_openwrt_os_image.sh https://downloads.openwrt.org/releases/21.02.2/targets/ramips/mt7621/openwrt-imagebuilder-21.02.2-ramips-mt7621.Linux-x86_64.tar.xz ubnt_unifi-6-lite
```

## Install the custom image
Custom-built images are installed the same way as normal OpenWrt images ([offical docs](https://openwrt.org/docs/guide-quick-start/factory_installation)).
1. Use `scp` to copy the custom image (should have the `.bin` file extension) to the router.
2. [Flash the firmware](https://openwrt.org/docs/guide-quick-start/factory_installation) using the image
3. TODO reset of instructions