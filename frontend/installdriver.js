import { Events, Window } from "@wailsio/runtime";
import { Backend, InstallDriverEvent } from "./bindings/changeme";

const state = {
  currentVersion: "",
  requiredVersion: "",
  init: function() {
    const params = new URLSearchParams(window.location.search);
    this.currentVersion = params.get("currentVersion") || "";
    this.currentVersion = params.get("requiredVersion") || "";
    document.dispatchEvent(new Event("Render", { bubbles: true }));
  },
  needInstall: function() {
    return this.currentVersion == "" || this.currentVersion.includes(this.requiredVersion);
  },
};

const infoMessage = document.getElementById("infoMessage");
if (!(infoMessage instanceof HTMLElement)) {
  throw new Error("element not found or invalid");
}
document.addEventListener("Render", function() {
  const needInstall = state.needInstall();
  infoMessage.textContent = needInstall ? "Driver is missing or out of date, please install" : "Driver is up to date";
});
document.addEventListener("InstallDriver", function() {
  infoMessage.textContent = "Installing...";
});
document.addEventListener("InstallDriverDone", function() {
  infoMessage.textContent = "Installation done";
});

const installDriverButton = document.getElementById("installDriverButton");
if (!(installDriverButton instanceof HTMLButtonElement)) {
  throw new Error("element not found or invalid");
}
document.addEventListener("Render", function() {
  const needInstall = state.needInstall();
  installDriverButton.style.display = needInstall ? "" : "none";
});
document.addEventListener("InstallDriver", function() {
  installDriverButton.disabled = true;
});
document.addEventListener("InstallDriverDone", function() {
  installDriverButton.disabled = false;
  installDriverButton.style.display = "none";
});

const installDriverButtonSpinner = installDriverButton.querySelector("svg");
if (!(installDriverButtonSpinner instanceof SVGSVGElement)) {
  throw new Error("element not found or invalid");
}
document.addEventListener("InstallDriver", function() {
  installDriverButtonSpinner.style.display = "";
});
document.addEventListener("InstallDriverDone", function() {
  installDriverButtonSpinner.style.display = "none";
});

const closeWindowButton = document.getElementById("closeWindowButton");
if (!(closeWindowButton instanceof HTMLButtonElement)) {
  throw new Error("element not found or invalid");
}
document.addEventListener("Render", function() {
  const needInstall = state.needInstall();
  closeWindowButton.style.display = needInstall ? "none" : "";
});
document.addEventListener("InstallDriverDone", function() {
  closeWindowButton.style.display = "";
});

document.addEventListener("CloseWindow", async function() {
  await Backend.CloseWindow(await Window.Name());
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

state.init();
