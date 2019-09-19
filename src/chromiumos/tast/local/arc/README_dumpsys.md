# Dumpsys maintenance guide

Rule of thumb:

*   Whenever possible, prefer Ash APIs, than using Dumpsys: Ash APIs are
    portable and don't require maintenance.
*   Dumpsys must work on all supported Android versions.
*   Prefer to parse Protobuf output than the text one: less error prone, easier
    to maintain.

## Quick intro to protobuf

`dumpsys` has the option to generate "text" output, and "protobuf" output, by
passing the `--proto` parameter.

*   "text" needs no further explanation.
*   "protobuf" is [Protocol Buffers][protocol-buffers] output.

Protobuf is a binary, structured format that is easy to parse. The format is
defined in `.proto` files. And the `.proto` files are used by `dumpsys` are
located here:

*   [frameworks/base/core/proto][frameworks-base]

[protocol-buffers]: https://developers.google.com/protocol-buffers/
[frameworks-base]: https://android.googlesource.com/platform/frameworks/base/+/refs/heads/android10-dev/core/proto/

These `.proto` files can be "compiled" to generate bindings for different
languages. Android compiles them to generate Java bindings. In Tast, we compile
them to generate Go bindings.

And we use the Go-generate bindings to parse the `dumpsys` output.

## Protobuf backward compatibility

The Protobuf protocol used by `dumpsys` is backwards compatible from an API
point of view. This means if message `X` is present on Android P, then it will
be present on Android Q, R, and so on. But take into account that some Protobuf
messages might get deprecated. This means that even if message `X` is available
in Android Q, if the message is deprecated, it might return `nil` data.

When a message gets deprecated, usually a new message that returns similar
information is added.

So your code might look like:

```go
if SDKVersion() == SDKP {
    // call message "X"
} else if SDKVersion() == SDKQ {
    // don't call message "X" since it is deprecated.
    // call "Y" instead.
}
```

## Debugging protobuf messages

Under normal circumstances, Protobuf should work without any issue. But if a
test reports something like the following:

    Protobuf message saved in test out directory. Filename: "activity-activities-protobuf-message-379545421.bin"
    failed to parse activity manager protobuf: proto: bad wiretype for field server.KeyguardControllerProto.KeyguardOccluded: got wiretype 2, want 0

It means that the protobuf message could be not understood. Probably because the
`.proto` files were taken from an older Android version than where the test is
running. E.g: The `.proto` files were taken from Android Q, while the test is
running in Android R.

In any case, it is good to debug what's happening. Grab the file
`activity-activities-protobuf-message-379545421.bin` from the error report and
do:

```sh
# Switch to the latest supported Android. And "cd" to Android root directory.
cd $ANDROID_SRC
# From there, decode the protobuf message.
protoc --decode com.android.server.am.ActivityManagerServiceDumpActivitiesProto frameworks/base/core/proto/android/server/activitymanagerservice.proto < protobuf-message-490183657.bin
```

Similarly, if you want to decode the output of `dumpsys activity --proto
activities`, do:

```sh
# As before, make sure that you are in the correct Android repo and directory:
cd $ANDROID_SRC
adb shell dumpsys activity --proto activities | protoc --decode com.android.server.am.ActivityManagerServiceDumpActivitiesProto frameworks/base/core/proto/android/server/activitymanagerservice.proto

# Fo "dumpsys window" you should do:
adb shell dumpsys window --proto | protoc --decode com.android.server.wm.WindowManagerServiceDumpProto frameworks/base/core/proto/android/server/windowmanagerservice.proto
```

If the output looks incomplete like the following:

```proto
window_configuration {
    app_bounds {
        right: 2400
        bottom: 1600
    }
    windowing_mode: 5

    # These results are incomplete. Value can be seen, but the var name not.
    # Something is wrong here!
    4 {
        3: 2400
        4: 1600
    }
}
```

...it means that there is a mismatch between the running Android, and from where
the proto files were taken from. If that is the case, you should update the
proto files:

*   [aosp-frameworks-base-proto.ebuild][aosp-frameworks-proto]

[aosp-frameworks-proto]: http://cs/chromeos_public/src/third_party/chromiumos-overlay/chromeos-base/aosp-frameworks-base-proto/

## AOSP-frameworks-base-proto

[aosp-frameworks-base-proto.ebuild][aosp-frameworks-proto] is the ebuild file
that does:

*   Fetches the `.proto` files from latest Android AOSP.
*   "Compiles" the `.proto` files to generate Go bindings.
*   Places the generated go bindings in ChromeOS SDKROOT so that it can be used
    in Tast tests.

In case you need to use `.proto` files from a new Android version, you just need
to update the `.ebuild` file.

*Caveat*: The files are taken from Android AOSP. If we need to start testing
Android R before it is published to Android-AOSP, we might need to take proto
files from Android internal (TBD).
