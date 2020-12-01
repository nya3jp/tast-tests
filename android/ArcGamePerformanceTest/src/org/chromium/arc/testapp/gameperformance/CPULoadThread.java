/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

/** Ballast thread that emulates CPU load by performing heavy computation in loop. */
public class CPULoadThread extends Thread {
    private boolean mStopRequest;

    public CPULoadThread() {
        mStopRequest = false;
    }

    private static double computePi() {
        double accumulator = 0;
        double prevAccumulator = -1;
        int index = 1;
        while (true) {
            accumulator += ((1.0 / (2.0 * index - 1)) - (1.0 / (2.0 * index + 1)));
            if (accumulator == prevAccumulator) {
                break;
            }
            prevAccumulator = accumulator;
            index += 2;
        }
        return 4 * accumulator;
    }

    // Requests thread to stop.
    public void issueStopRequest() {
        synchronized (this) {
            mStopRequest = true;
        }
    }

    @Override
    public void run() {
        // Load CPU by PI computation.
        while (computePi() != 0) {
            synchronized (this) {
                if (mStopRequest) {
                    break;
                }
            }
        }
    }
}
