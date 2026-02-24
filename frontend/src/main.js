import {Events} from "@wailsio/runtime";
import {GreetService} from "../bindings/changeme";
import "basecoat-css/basecoat";
import "basecoat-css/all";

const resultElement = document.getElementById('result');
const timeElement = document.getElementById('time');

window.doGreet = async () => {
    let name = document.getElementById('name').value;
    if (!name) {
        name = 'anonymous';
    }
    try {
        resultElement.innerText = await GreetService.Greet(name);
    } catch (err) {
        console.error(err);
    }
}

Events.On('time', (time) => {
    // timeElement.innerText = time.data;
});

/**
 * @type {Record<string, (element: Element, attributeValue: string) => void>}
 */
const initFunctions = {
  "data-click-event": function initClickEvent(targetElement, attributeValue) {
    targetElement.addEventListener("click", function dispatchEvent() {
      document.dispatchEvent(new Event(attributeValue));
    });
  },
  "data-go-back": function initGoBack(targetElement) {
    if (targetElement.tagName != "A") {
      return;
    }
    targetElement.addEventListener("click", function goBack(event) {
      if (!(event instanceof PointerEvent)) {
        return;
      }
      if (document.referrer && history.length > 2 && !event.ctrlKey && !event.metaKey) {
        event.preventDefault();
        history.back();
      }
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
