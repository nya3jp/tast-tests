/*
 * Copyright 2022 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.inputoverlay;

import android.view.InputDevice;
import android.view.InputEvent;
import android.view.KeyEvent;
import android.view.MotionEvent;

class ReceivedEvent {
  public Long eventTime;
  public Long receiveTimeNs;
  public String source;
  public String code;
  public String action;
  public String actionButton;
  public ReceivedEvent(InputEvent event, Long receiveTimeNs) {
    this.eventTime = event.getEventTime();
    this.receiveTimeNs = receiveTimeNs;
    source = sourceToString(event);
    code = codeToString(event);
    action = actionToString(event);
    actionButton = actionButtonToString(event);
  }

  public static String sourceToString(InputEvent event) {
    int source = event.getSource();
    switch (source) {
      case InputDevice.SOURCE_KEYBOARD:
        return "keyboard";
      case InputDevice.SOURCE_JOYSTICK:
        return "joystick";
      case InputDevice.SOURCE_GAMEPAD:
        return "gamepad";
      case InputDevice.SOURCE_TOUCHSCREEN:
        return "touchscreen";
      case InputDevice.SOURCE_TOUCHPAD:
        return "touchpad";
      case InputDevice.SOURCE_MOUSE:
        return "mouse";
      default:
        return "Unsupported source";
    }
  }

  public static String codeToString(InputEvent event) {
    if (event instanceof KeyEvent) {
      int code = ((KeyEvent) event).getKeyCode();
      return KeyEvent.keyCodeToString(code);
    }
    return KeyEvent.keyCodeToString(0);
  }

  public static String actionToString(InputEvent event) {
    if (event instanceof MotionEvent) {
      int action = ((MotionEvent) event).getActionMasked();
      return MotionEvent.actionToString(action);
    } else if (event instanceof KeyEvent) {
      int action = ((KeyEvent) event).getAction();
      switch (action) {
        case KeyEvent.ACTION_DOWN:
          return "ACTION_DOWN";
        case KeyEvent.ACTION_UP:
          return "ACTION_UP";
        default:
          return "unknown";
      }
    } else {
      return "unknown";
    }
  }

  // Only consider the mouse primary button and secondary button.
  public static String actionButtonToString(InputEvent event) {
    if (event instanceof MotionEvent) {
      int actionBtn = ((MotionEvent) event).getActionButton();
      if ((actionBtn & MotionEvent.BUTTON_PRIMARY) != 0) {
        return "BUTTON_PRIMARY";
      }
      if ((actionBtn & MotionEvent.BUTTON_SECONDARY) != 0 ) {
        return "BUTTON_SECONDARY";
      }
    }
    return "0";
  }

}
