/*
 * Copyright 2022 The ChromiumOS Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.inputoverlay;

import android.view.InputDevice;
import android.view.InputEvent;
import android.view.KeyEvent;
import android.view.MotionEvent;

// ReceivedEvent includes the input event readable info received by the app.
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

  public String toString() {
    return action + " " + String.valueOf(receiveTimeNs);
  }

   private String sourceToString(InputEvent event) {
    final int source = event.getSource();
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

  // KeyEvent code to string.
  private static String codeToString(InputEvent event) {
    if (event instanceof KeyEvent) {
      final int code = ((KeyEvent) event).getKeyCode();
      return KeyEvent.keyCodeToString(code);
    }
    // For event is not KeyEvent.
    return "N/A keycode";
  }

  // Event action to string.
  private static String actionToString(InputEvent event) {
    if (event instanceof MotionEvent) {
      final int action = ((MotionEvent) event).getActionMasked();
      return MotionEvent.actionToString(action);
    } else if (event instanceof KeyEvent) {
      final int action = ((KeyEvent) event).getAction();
      switch (action) {
        case KeyEvent.ACTION_DOWN:
          return "ACTION_DOWN";
        case KeyEvent.ACTION_UP:
          return "ACTION_UP";
        default:
          return "N/A action";
      }
    } else {
      return "N/A action";
    }
  }

  // MotionEvent action button to string. Only consider the mouse primary button and secondary
  // button for now.
  private static String actionButtonToString(InputEvent event) {
    if (event instanceof MotionEvent) {
      final int actionBtn = ((MotionEvent) event).getActionButton();
      if ((actionBtn & MotionEvent.BUTTON_PRIMARY) != 0) {
        return "BUTTON_PRIMARY";
      }
      if ((actionBtn & MotionEvent.BUTTON_SECONDARY) != 0 ) {
        return "BUTTON_SECONDARY";
      }
    }
    return "N/A action_button";
  }

}
