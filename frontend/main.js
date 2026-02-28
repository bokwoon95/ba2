import { Events } from "@wailsio/runtime";
import { BackendService } from "./bindings/changeme";
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
  console.log(await BackendService.Hello());
})();

/**
 * @type {Record<string, (element: Element, attributeValue: string) => void>}
 */
const initFunctions = {
  "data-click-event": function initClickEvent(targetElement, attributeValue) {
    targetElement.addEventListener("click", function dispatchEventOnClick() {
      document.dispatchEvent(new Event(attributeValue));
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
