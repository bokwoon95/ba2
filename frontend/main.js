import { Events } from "@wailsio/runtime";
import { Backend, UpdateEvent } from "./bindings/changeme";
import "basecoat-css/basecoat";
import "basecoat-css/all";

Events.On("time", (time) => {
  // timeElement.innerText = time.data;
});

(async function init() {
  let response = await fetch("/backend/driver/");
  if (!response.ok) {
    console.error(response);
  } else {
    const driverData = await response.json();
    console.log(driverData);
  }
  console.log(await Backend.Hello());
})();

/**
 * streamResponseLines is a generator function that streams over a response
 * body (returned by fetch()) line by line. Use it like this:
 * `for await (const line of streamResponseLines(response)) { ... }`.
 *
 * @param {Response} response
 */
async function* streamResponseLines(response) {
  const reader = response.body.getReader();
  const textDecoder = new TextDecoder("utf-8");
  let line = "";
  let chunk = new Uint8Array();
  // Read from the response body in chunks.
  for (let readResult = await reader.read(); !readResult.done; readResult = await reader.read()) {
    if (chunk.length > 0) {
      // We have a carryover chunk from a previous iteration. There are
      // guaranteed to be no newlines inside since the previous iteration's
      // chunk.indexOf(10) would have caught it.
      //
      // Stream option has to be true because we haven't encountered a
      // newline yet, more data may be decoded for the current line.
      line += textDecoder.decode(chunk, { stream: true });
    }
    // Get the reader's current chunk.
    chunk = readResult.value;
    // Jump to each newline '\n' byte in the chunk. 10 is the ASCII/UTF-8
    // decimal value of the '\n' byte.
    for (let index = chunk.indexOf(10); index >= 0; index = chunk.indexOf(10)) {
      // We found a newline, decode everything up to this index and consider
      // that as a complete line and yield it.
      line += textDecoder.decode(chunk.subarray(0, index));
      yield line;
      // Reset the line.
      line = "";
      // Shorten the chunk to exclude what we have already decoded.
      chunk = chunk.subarray(index + 1);
    }
  }
  // Flush any remainder bytes in the chunk.
  if (chunk.length > 0) {
    line += textDecoder.decode(chunk);
    yield line;
  }
}
const textarea = document.getElementById("textarea");
document.addEventListener("backend:installdriver", async function() {
  console.log("received backend:installdriver");
  textarea.value = "";
  const eventID = Math.random().toString(36).substring(2);
  const response = await fetch(`/backend/installdriver/?eventID=${eventID}`, { method: "POST" });
  console.log(response);
  if (response.ok) {
    const unregister = Events.On("backend:update", function(event) {
      if (event.data.eventID != eventID) {
        return;
      }
      textarea.value += `${event.data.category}: ${event.data.message}\n`;
      if (event.data.category == "error" || event.data.category == "success") {
        unregister();
      }
    });
  }
});

/**
 * @type {Record<string, (element: Element, attributeValue: string) => void>}
 */
const initFunctions = {
  "data-click-event": function initClickEvent(targetElement, attributeValue) {
    targetElement.addEventListener("click", function dispatchEventOnClick() {
      console.log("dispatched " + attributeValue);
      document.dispatchEvent(new Event(attributeValue, { bubbles: true }));
    });
  },
};
const attributeNames = Object.keys(initFunctions);
/**
 * @param {Element} targetElement
 */
function initialize(targetElement) {
  for (const attributeName of attributeNames) {
    if (targetElement.hasAttribute(attributeName) && !targetElement.hasAttribute(attributeName + "-initialized")) {
      try {
        initFunctions[attributeName](targetElement, targetElement.getAttribute(attributeName));
      } catch (e) {
        console.error(e);
      }
      targetElement.setAttribute(attributeName + "-initialized", "");
    }
  }
}
const selector = attributeNames.map(name => "[" + name + "]").join(", ");
for (const targetElement of document.querySelectorAll(selector)) {
  initialize(targetElement);
}
const observer = new MutationObserver(function(mutationRecords) {
  for (const mutationRecord of mutationRecords) {
    if (mutationRecord.type != "childList") {
      continue;
    }
    for (const addedElement of mutationRecord.addedNodes) {
      if (!(addedElement instanceof Element)) {
        continue;
      }
      initialize(addedElement);
      for (const targetElement of targetElement.querySelectorAll(selector)) {
        if (!(targetElement instanceof Element)) {
          continue;
        }
        initialize(targetElement);
      }
    }
  }
});
observer.observe(document.body, {
  childList: true,
  subtree: true,
});
