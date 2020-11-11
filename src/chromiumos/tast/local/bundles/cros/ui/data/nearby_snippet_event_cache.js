/**
 * @fileoverview Define custom functions to let test scripts to query
 * snippet event.
 *
 * about cr.define
 * https://source.chromium.org/chromium/chromium/src/+/master:ui/webui/resources/js/cr.js?q=function%5C%20define%5C(
 */

// Define the functions for event cache:
// nearbySnippetEventCache.getEvent
// nearbySnippetEventCache.postEvent
let nearbySnippetEventCache = function() {
  /** @private {Map!} */
  let eventQueue_ = new Map();

  /**
   * Gets and removes an event of a certain name that has been received so far.
   * @param {string} eventName
   * @return {string?} event data in JSON format.
   */
  function getEvent(eventName) {
    if (eventQueue_.has(eventName)) {
      return JSON.stringify(eventQueue_.get(eventName).pop());
    }
  }

  /**
   * Post a data object to the Event cache of the certain event name.
   * @param {string} eventName
   * @param {Object!} eventData
   */
  function postEvent(eventName, eventData) {
    if (eventQueue_.has(eventName)) {
      eventQueue_.get(eventName).unshift(eventData);
    } else {
      eventQueue_.set(eventName, [eventData]);
    }
  }

  // #cr_define_end
  return {getEvent: getEvent, postEvent: postEvent};
}();
