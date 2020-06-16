/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.inputlatency;

import android.view.InputEvent;
import android.view.KeyEvent;
import android.view.MotionEvent;

import org.json.JSONObject;
import org.json.JSONException;

public class ReceivedEvent {
    public InputEvent event;
    public Long receiveTime;
    public Long rtcReceiveTime;
    public Long eventTime;
    public Long latency;
    public String type;

    public ReceivedEvent(InputEvent event, Long receiveTime, Long rtcReceiveTime) {
        // Note that on ARC++, eventTime is the same as the original (host) kernel
        // timestamp of the event. On ARCVM, the event timestamp is rewritten in the
        // guest kernel due to differing monotonic clocks (b/123416853).
        this.event = event;
        this.eventTime = event.getEventTime();
        this.receiveTime = receiveTime;
        this.rtcReceiveTime = rtcReceiveTime;
        this.latency = receiveTime - eventTime;

        if (event instanceof KeyEvent) {
            type = "KeyEvent";
        } else if (event instanceof MotionEvent) {
            type = "MotionEvent";
        } else {
            type = "InputEvent";
        }
    }

    public JSONObject toJSON() throws JSONException {
        return new JSONObject()
                .put("type", type)
                .put("eventTime", eventTime)
                .put("receiveTime", receiveTime)
                .put("rtcReceiveTime", rtcReceiveTime)
                .put("latency", latency);
    }

    @Override
    public String toString() {
        return String.format(
                "%s:%d:%d:%d:%d", type, eventTime, receiveTime, rtcReceiveTime, latency);
    }
}
