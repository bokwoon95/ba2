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

const statusBarElement = document.getElementById("statusBar");
if (!(statusBarElement instanceof HTMLElement)) {
  throw new Error("element not found or invalid");
}
/**
 * @type {{
 *  element: HTMLElement,
 *  processes: StatusBarEvent[][],
 *  processIndex: Map<string, number>,
 * }}
 */
const statusBar = {
  element: statusBarElement,
  processes: [],
  processIndex: new Map(),
}
Events.On("StatusBarEvent", async function(event) {
  const windowName = await Window.Name();
  if (event.sender != windowName) {
    return;
  }
  const statusBarEvent = new StatusBarEvent(event.data);
  const index = statusBar.processIndex.get(statusBarEvent.eventID);
  if (index == null) {
    if (statusBarEvent.category != "stop") {
      statusBar.processes.push([statusBarEvent]);
      statusBar.processIndex.set(statusBarEvent.eventID, statusBar.processes.length - 1);
      document.dispatchEvent(new Event("StatusBarStateUpdated", { bubbles: true }));
    }
  } else {
    if (statusBarEvent.category == "stop") {
      // TODO: mark this process as tombstoned.
      return;
    }
    statusBar.processes[index].push(statusBarEvent);
  }
});
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
