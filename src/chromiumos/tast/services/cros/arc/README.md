# protocol buffer definitions

## Building

When editing protocol buffers, these are not regenerated with `tast run`
invocation. Run the following command in CrOS chroot to regenerate protocol
buffer bindings:

```shell
~/trunk/src/platform/tast/tools/go.sh generate chromiumos/tast/services/cros/arc
```
