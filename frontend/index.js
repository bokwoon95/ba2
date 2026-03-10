import { Events, Window } from "@wailsio/runtime";
import { Backend, StatusBarEvent, InstallDriverEvent, WebviewWindowOptions, MessageDialogOptions } from "./bindings/changeme";
import "basecoat-css/basecoat";
import "basecoat-css/all";

async function init() {
  try {
    let response = await fetch("/backend/driver/");
    const driverData = await response.json();
    if (!driverData.currentVersion.includes(driverData.requiredVersion)) {
      const params = new URLSearchParams();
      params.append("currentVersion", driverData.currentVersion);
      params.append("requiredVersion", driverData.requiredVersion);
      await Backend.CreateWindow(new WebviewWindowOptions({
        Name: "installdriver",
        URL: `/installdriver.html?${params.toString()}`,
      }));
      Backend.EnableWindow("main", false);
      await new Promise(function(resolve) {
        const unregister = Events.On("WindowClosed", function(event) {
          if (event.sender != "installdriver") {
            return;
          }
          unregister();
          resolve(null);
        });
      });
      Backend.EnableWindow("main", true);
      Backend.FocusWindow("main");
    }
    await Backend.StartPlaywright();
    await Backend.OpenBrowser();
    console.log(await Backend.Hello());
  } catch (err) {
    Backend.Dialog(new MessageDialogOptions({
      Title: "Error",
      Message: err instanceof Error ? err.toString() : String(err),
    }));
  }
}

const statusBar = document.getElementById("statusBar");
if (!(statusBar instanceof HTMLElement)) {
  throw new Error("element not found or invalid");
}
const statusBarState = {
  /** @type {StatusBarEvent[]} */
  processStack: [],
  /** @type {Map<string, {index: number, tombstoned: boolean, startedAt: number}>} */
  processMetadata: new Map(),
}
Events.On("StatusBarEvent", async function(event) {
  const windowName = await Window.Name();
  if (event.sender != windowName) {
    return;
  }
  const metadata = statusBarState.processMetadata.get(event.data.eventID);
  if (metadata == null) {
    if (event.data.done) {
      return;
    }
    statusBarState.processStack.push(event.data);
    statusBarState.processMetadata.set(event.data.eventID, {
      index: statusBarState.processStack.length - 1,
      tombstoned: false,
      startedAt: event.data.timestamp,
    });
    document.dispatchEvent(new Event("StatusBarStateUpdated", { bubbles: true }));
    return;
  }
  if (event.data.done) {
    metadata.tombstoned = true;
    document.dispatchEvent(new Event("StatusBarStateUpdated", { bubbles: true }));
    return;
  }
  statusBarState.processStack[metadata.index] = event.data;
  document.dispatchEvent(new Event("StatusBarStateUpdated", { bubbles: true }));
  return;
});

const statusBarSpinner = statusBar.querySelector("svg");
if (!(statusBarSpinner instanceof SVGSVGElement)) {
  throw new Error("element not found or invalid");
}
// TODO: register an init function which is called at the bottom which hides or shows the statusBarSpinner.

const statusBarStatus = statusBar.querySelector("[role=status]");
if (!(statusBarStatus instanceof HTMLElement)) {
  throw new Error("element not found or invalid");
}
// TODO: register an init function which is called at the bottom which hides or shows the statusBarStatus message.
document.addEventListener("StatusBarStateUpdated", function() {
  // TODO: consult statusBar state and update textContent accordingly.
});


document.addEventListener("Connect", function() {
  Backend.Dialog(new MessageDialogOptions({
    Title: "Info",
    Message: "Hi there",
  }));
});


const textarea = document.getElementById("textarea");
if (!(textarea instanceof HTMLTextAreaElement)) {
  throw new Error("element not found or invalid");
}
document.addEventListener("InstallDriver", async function() {
  const windowName = await Window.Name();
  const promise = fetch(`/backend/installdriver/?windowName=${windowName}`, { method: "POST" });
  let stickToBottom = true;
  const updateStickToBottom = function() {
    stickToBottom = textarea.scrollHeight - textarea.scrollTop - textarea.clientHeight <= 50 /* px tolerance */;
  }
  textarea.addEventListener("scroll", updateStickToBottom);
  textarea.value = "";
  const unregister = Events.On("InstallDriverEvent", function(event) {
    if (event.sender != windowName) {
      return;
    }
    const installDriverEvent = new InstallDriverEvent(event.data);
    textarea.value += `${installDriverEvent.category}: ${installDriverEvent.message}\n`;
    if (stickToBottom) {
      textarea.scrollTop = textarea.scrollHeight;
    }
  });
  try {
    await promise;
  } finally {
    unregister();
    textarea.removeEventListener("scroll", updateStickToBottom);
    document.dispatchEvent(new Event("InstallDriverDone", { bubbles: true }));
  }
});

await init();
