import { Events } from "@wailsio/runtime";
import {
  Backend,
  UpdateEvent,
  WebviewWindowOptions,
  MessageDialogOptions,
} from "./bindings/changeme";
import "basecoat-css/basecoat";
import "basecoat-css/all";

(async function init() {
  let response = await fetch("/backend/driver/");
  if (!response.ok) {
    Backend.Dialog(new MessageDialogOptions({
      Message: `fetching driver details: ${response.statusText}`,
    }));
  } else {
    const driverData = await response.json();
    console.log(driverData);
    if (driverData.currentVersion.includes(driverData.requiredVersion)) {
      await Backend.CreateWindow(new WebviewWindowOptions({
        Name: "installdriver",
        URL: "/installdriver.html",
      }));
      console.log("installdriver spawned");
      Backend.EnableWindow("main", false);
      await new Promise(function(resolve) {
        const unregister = Events.On("backend:windowclosed", function(event) {
          if (event.sender != "installdriver") {
            return;
          }
          unregister();
          resolve();
        });
      });
      console.log("installdriver closed");
      Backend.EnableWindow("main", true);
      Backend.FocusWindow("main");
    }
  }
  console.log(await Backend.Hello());
})();

document.addEventListener("backend:connect", function() {
  Backend.Dialog(new MessageDialogOptions({
    Title: "Info",
    Message: "Hi there",
  }));
});

const textarea = document.getElementById("textarea");
document.addEventListener("backend:installdriver", async function() {
  const eventID = Math.random().toString(36).substring(2);
  const promise = fetch(`/backend/installdriver/?eventID=${eventID}`, { method: "POST" });
  let stickToBottom = true;
  function updateStickToBottom() {
    stickToBottom = textarea.scrollHeight - textarea.scrollTop - textarea.clientHeight <= 50 /* px tolerance */;
  }
  textarea.addEventListener("scroll", updateStickToBottom);
  textarea.value = "";
  const unregister = Events.On("backend:update", function(event) {
    const updateEvent = new UpdateEvent(event.data);
    if (updateEvent.eventID != eventID) {
      return;
    }
    textarea.value += `${updateEvent.category}: ${updateEvent.message}\n`;
    if (stickToBottom) {
      textarea.scrollTop = textarea.scrollHeight;
    }
  });
  try {
    await promise;
  } finally {
    unregister();
    textarea.removeEventListener("scroll", updateStickToBottom);
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
