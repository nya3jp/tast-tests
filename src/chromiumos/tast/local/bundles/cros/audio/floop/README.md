# How to read audio.CrasFloop.* test results

Each parameterized test is run with a different schedule to check all kinds of lifecycle combinations.

## Timeline

The timeline shows what will happen at which time if everything goes smoothly.

Take the output of audio.CrasFloop.rpbc as an example:

```
scheduled test timeline:
   --- time -->: 0123456789
   floop active: ffffffffff
        capture: ----cccccc
       playback: --ppppp---
   check volume: -----1----
     check zero: --------0-
```

-   The flexible loopback device will be requested at Δt=0s. (Δt means time relative to test start)
    Currently for all cases, once enabled, the flexible loopback device will be active throughout the test.
-   At Δt=4, a client will capture audio from floop for 6 seconds until Δt=10.
-   At Δt=2, a client will play audio for 5 seconds until Δt=7.
-   For the captured audio clip, the sample during Δt=(5,6) should match the playback.
-   For the captured audio clip, the cample during Δt=(8,9) should have no sound.

## Actual timings

Followed by the timeline above, is the actual timings & logs of the events during the test.

```
Δt = 59.18µs: request floop (on schedule)
floop device id: 13
Δt = 2.000368165s: start playback (on schedule)
Δt = 4.000180527s: start capture (on schedule)
Δt = 8.001383734s: end playback (1.001383734s overdue)
Error at floop.go:268: Playback failed: context deadline exceeded
```

From the logs we can observe that according to the timings the playback should end at Δt=7
but keeps running until Δt=8, when it is killed.


## Common failure messages

-   **{Playback/Capture} failed: context deadline exceeded**

    Playback or capture did not end on time. It was probably blocked.

-   **Unexpected %s dB in Δt=(%d,%d); wav time=(%d,%d): want %f, got %f**

    The captured audio does not have the correct volume.
    Either the captured clip has sound when it shouldn't be, or the reverse,
    or the volume is simply incorrect.
    *Δt* is the time relative to the test time. *wav time* is the time relative to capture.wav.
