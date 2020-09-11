/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.inputlatency;

import android.view.InputDevice;
import android.view.InputEvent;
import android.view.KeyEvent;
import android.view.MotionEvent;

import org.json.JSONException;
import org.json.JSONObject;

public class ReceivedEvent {
    public InputEvent event;
    public Long receiveTimeNs;
    public String source;
    public String code;
    public String action;

    public ReceivedEvent(InputEvent event, Long receiveTimeNs) {
        // Note that on ARC++, eventTime is the same as the original (host) kernel
        // timestamp of the event. On ARCVM, the event timestamp is rewritten in the
        // guest kernel due to differing monotonic clocks (b/123416853).
        this.event = event;
        this.receiveTimeNs = receiveTimeNs;

        switch (event.getSource()) {
            case InputDevice.SOURCE_KEYBOARD:
                source = "Keyboard";
                break;
            case InputDevice.SOURCE_JOYSTICK:
                source = "Joystick";
                break;
            case InputDevice.SOURCE_GAMEPAD:
                source = "Gamepad";
                break;
            case InputDevice.SOURCE_MOUSE:
                source = "Mouse";
                break;
            case InputDevice.SOURCE_STYLUS:
                source = "Stylus";
                break;
            case InputDevice.SOURCE_TOUCHPAD:
                source = "Touchpad";
                break;
            case InputDevice.SOURCE_TOUCHSCREEN:
                source = "Touchscreen";
                break;
            default:
                source = "UnsupportedSource";
        }

        if (event instanceof KeyEvent) {
            code = KeyEvent.keyCodeToString(((KeyEvent)event).getKeyCode());
            action = actionToString(((KeyEvent)event).getAction());
        } else if (event instanceof MotionEvent) {
            code = source;
            action = MotionEvent.actionToString(((MotionEvent)event).getAction());
        } else {
            code = "UnsupportedEvent";
            action = "UnsupportedEvent";
        }
    }

    private String actionToString(int action) {
        switch (action) {
            case KeyEvent.ACTION_DOWN:
                return "ACTION_DOWN";
            case KeyEvent.ACTION_UP:
                return "ACTION_UP";
            case KeyEvent.ACTION_MULTIPLE:
                return "ACTION_MULTIPLE";
            default:
                return Integer.toString(action);
        }
    }

    public JSONObject toJSON() throws JSONException {
        return new JSONObject()
                .put("source", source)
                .put("code", code)
                .put("action", action)
                .put("receiveTimeNs", receiveTimeNs);
    }

    @Override
    public String toString() {
        return String.format(
                "%s:%s:%s:%d",
                source, code, action, receiveTimeNs);
    }
}
