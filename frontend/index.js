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
  try {
    let response = await fetch("/backend/driver/");
    if (!response.ok) {
      Backend.Dialog(new MessageDialogOptions({
        Message: `fetching driver details: ${response.statusText}`,
      }));
    } else {
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
            resolve();
          });
        });
        Backend.EnableWindow("main", true);
        Backend.FocusWindow("main");
      }
    }
    await Backend.StartPlaywright();
    await Backend.OpenBrowser();
    console.log(await Backend.Hello());
  } catch (e) {
    Backend.Dialog(new MessageDialogOptions({
      Title: "Error",
      Message: e,
    }));
  }
})();

document.addEventListener("Connect", function() {
  Backend.Dialog(new MessageDialogOptions({
    Title: "Info",
    Message: "Hi there",
  }));
});

const textarea = document.getElementById("textarea");
document.addEventListener("InstallDriver", async function() {
  const eventID = Math.random().toString(36).substring(2);
  const promise = fetch(`/backend/installdriver/?eventID=${eventID}`, { method: "POST" });
  let stickToBottom = true;
  function updateStickToBottom() {
    stickToBottom = textarea.scrollHeight - textarea.scrollTop - textarea.clientHeight <= 50 /* px tolerance */;
  }
  textarea.addEventListener("scroll", updateStickToBottom);
  textarea.value = "";
  const unregister = Events.On("UpdateEvent", function(event) {
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
