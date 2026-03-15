import { Events, Window } from "@wailsio/runtime";
import { Backend, ProcessUpdate, InstallDriverEvent, WebviewWindowOptions, MessageDialogOptions } from "./bindings/changeme";
import "basecoat-css/basecoat";
import "basecoat-css/all";

(async function init() {
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
      await Backend.EnableWindow("main", false);
      await new Promise(function(resolve) {
        const unregister = Events.On("WindowClosed", function(event) {
          if (event.sender != "installdriver") {
            return;
          }
          unregister();
          resolve(null);
        });
      });
      await Backend.EnableWindow("main", true);
      await Backend.FocusWindow("main");
    }
    await Backend.StartPlaywright();
    await Backend.OpenBrowser();
    console.log(await Backend.Hello());
  } catch (err) {
    console.error(err);
    await Backend.Dialog(new MessageDialogOptions({
      Title: "Error",
      Message: err instanceof Error ? err.message : String(err),
    }));
  }
})();

/**
 * @type {Set<string>}
 */
const initEvents = new Set();
try {
  const statusBar = document.getElementById("statusBar");
  if (!(statusBar instanceof HTMLElement)) {
    throw new Error("element not found or invalid");
  }
  const statusBarState = {
    /** @type {ProcessUpdate[]} */
    processStack: [],
    /** @type {Map<string, {index: number, tombstoned: boolean, startedAt: number}>} */
    processInfoMap: new Map(),
    /** @type {ProcessUpdate | null} */
    currentProcess: null,
    /** @param {ProcessUpdate} processUpdate */
    pushProcessUpdate: function(processUpdate) {
      for (let i = this.processStack.length - 1; i >= 0; i = this.processStack.length - 1) {
        const processUpdate = this.processStack[i];
        const processInfo = this.processInfoMap.get(processUpdate.processID);
        if (processInfo != null && !processInfo.tombstoned) {
          break;
        }
        this.processStack.pop();
        this.processInfoMap.delete(processUpdate.processID);
      }
      const processInfo = this.processInfoMap.get(processUpdate.processID);
      if (processInfo == null) {
        if (processUpdate.progressValue >= processUpdate.progressMax) {
          return;
        }
        this.processStack.push(processUpdate);
        this.processInfoMap.set(processUpdate.processID, {
          index: this.processStack.length - 1,
          tombstoned: false,
          startedAt: processUpdate.timestamp,
        });
        this.currentProcess = processUpdate;
        document.dispatchEvent(new Event("CurrentProcessUpdated", { bubbles: true }));
        return;
      }
      if (processUpdate.progressValue >= processUpdate.progressMax) {
        if (processInfo.index == this.processStack.length - 1) {
          this.processStack.pop();
        }
        processInfo.tombstoned = true;
        document.dispatchEvent(new Event("CurrentProcessUpdated", { bubbles: true }));
        return;
      }
      this.processStack[processInfo.index] = processUpdate;
      document.dispatchEvent(new Event("CurrentProcessUpdated", { bubbles: true }));
      return;
    },
  }
  Events.On("ProcessUpdate", async function(event) {
    const windowName = await Window.Name();
    if (event.sender != windowName) {
      return;
    }
    statusBarState.pushProcessUpdate(event.data);
  });

  const statusBarSpinner = statusBar.querySelector("svg");
  if (!(statusBarSpinner instanceof SVGSVGElement)) {
    throw new Error("element not found or invalid");
  }
  initEvents.add("CurrentProcessUpdated");
  document.addEventListener("CurrentProcessUpdated", function() {
  });
  // TODO: register an init function which is called at the bottom which hides or shows the statusBarSpinner.

  const statusBarStatus = statusBar.querySelector("[role=status]");
  if (!(statusBarStatus instanceof HTMLElement)) {
    throw new Error("element not found or invalid");
  }
  // TODO: register an init function which is called at the bottom which hides or shows the statusBarStatus message.
  document.addEventListener("CurrentProcessUpdated", function() {
    // TODO: consult statusBar state and update textContent accordingly.
  });

  document.addEventListener("Connect", async function() {
    await Backend.Dialog(new MessageDialogOptions({
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
} finally {
  for (const initEvent of initEvents) {
    document.dispatchEvent(new Event(initEvent, { bubbles: true }));
  }
}
