import { Events, Window } from "@wailsio/runtime";
import { Backend, UpdateEvent } from "./bindings/changeme";

const currentVersionField = document.getElementById("currentVersion");
const requiredVersionField = document.getElementById("requiredVersion");
function initialize() {
  const params = new URLSearchParams(window.location.search);
  currentVersionField.value = params.get("currentVersion");
  requiredVersionField.value = params.get("requiredVersion");
  document.dispatchEvent(new Event("frontend:render", { bubbles: true }));
}

const infoMessage = document.getElementById("infoMessage");
document.addEventListener("frontend:render", function() {
  const needInstall = currentVersionField.value == "" || !currentVersionField.value.includes(requiredVersionField.value) || true;
  infoMessage.textContent = needInstall ? "Driver is missing or out of date, please install" : "Driver is up to date";
});
document.addEventListener("frontend:installdriver", function() {
  infoMessage.textContent = "Installing...";
});
document.addEventListener("frontend:installdriverdone", function() {
  infoMessage.textContent = "Installation done";
});

const installDriverButton = document.getElementById("installDriverButton");
document.addEventListener("frontend:render", function() {
  const needInstall = currentVersionField.value == "" || !currentVersionField.value.includes(requiredVersionField.value) || true;
  installDriverButton.style.display = needInstall ? "" : "none";
});
document.addEventListener("frontend:installdriver", function() {
  installDriverButton.disabled = true;
});
document.addEventListener("frontend:installdriverdone", function() {
  installDriverButton.disabled = false;
  installDriverButton.style.display = "none";
});

const installDriverButtonSpinner = installDriverButton.querySelector("svg");
document.addEventListener("frontend:installdriver", function() {
  installDriverButtonSpinner.style.display = "";
});
document.addEventListener("frontend:installdriverdone", function() {
  installDriverButtonSpinner.style.display = "none";
});

const closeWindowButton = document.getElementById("closeWindowButton");
document.addEventListener("frontend:render", function() {
  const needInstall = currentVersionField.value == "" || !currentVersionField.value.includes(requiredVersionField.value) || true;
  closeWindowButton.style.display = needInstall ? "none" : "";
});
document.addEventListener("frontend:installdriverdone", function() {
  closeWindowButton.style.display = "";
});

document.addEventListener("frontend:closewindow", async function() {
  Backend.CloseWindow(await Window.Name());
});

const textarea = document.getElementById("textarea");
document.addEventListener("frontend:installdriver", async function() {
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
    document.dispatchEvent(new Event("frontend:installdriverdone", { bubbles: true }));
  }
});

initialize();
