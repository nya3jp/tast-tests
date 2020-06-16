/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.inputlatency;

import android.view.InputEvent;
import android.view.KeyEvent;
import android.view.MotionEvent;

public class ReceivedEvent {
    public InputEvent event;
    public Long receiveTime;
    public Long kernelTime;
    public Long latency;
    public String type;

    public ReceivedEvent(InputEvent event, Long receiveTime) {
        this.event = event;
        this.receiveTime = receiveTime;
        kernelTime = event.getEventTime();
        latency = receiveTime - kernelTime;
        if (event instanceof KeyEvent) type = "KeyEvent";
        else if (event instanceof MotionEvent) type = "MotionEvent";
        else type = "InputEvent";
    }

    @Override
    public String toString() {
        return String.format("%s:%d:%d:%d", type, kernelTime, receiveTime, latency);
    }
}
