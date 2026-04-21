const state = {
  info: null,
  devices: [],
  transfers: [],
  selectedDeviceId: "",
  selectedFiles: [],
  pollingStarted: false,
};

const refs = {
  statusPill: document.getElementById("statusPill"),
  deviceLabel: document.getElementById("deviceLabel"),
  deviceCount: document.getElementById("deviceCount"),
  deviceList: document.getElementById("deviceList"),
  selectedFiles: document.getElementById("selectedFiles"),
  transferList: document.getElementById("transferList"),
  pickFilesBtn: document.getElementById("pickFilesBtn"),
  sendBtn: document.getElementById("sendBtn"),
};

function appApi() {
  return window.go && window.go.main && window.go.main.App ? window.go.main.App : null;
}

function setStatus(message) {
  refs.statusPill.textContent = message;
}

function fileNameFromPath(path) {
  return path.split(/[/\\]/).pop() || path;
}

function bytesToText(bytes) {
  const units = ["B", "KB", "MB", "GB"];
  let value = Number(bytes) || 0;
  let idx = 0;
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024;
    idx += 1;
  }
  return `${value.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
}

function percent(progress) {
  const p = Math.max(0, Math.min(100, Math.round((Number(progress) || 0) * 100)));
  return `${p}%`;
}

function renderDevices() {
  refs.deviceCount.textContent = `${state.devices.length} online`;

  if (state.devices.length === 0) {
    refs.deviceList.innerHTML = '<div class="subtle">No online devices discovered yet...</div>';
    return;
  }

  refs.deviceList.innerHTML = state.devices
    .map((device) => {
      const isSelected = state.selectedDeviceId === device.id;
      const latency = Number.isFinite(device.latency) ? `${device.latency} ms` : "-";
      return `
        <button class="device-item ${isSelected ? "selected" : ""}" data-device-id="${device.id}">
          <div class="device-title">
            <strong>${escapeHtml(device.name)}</strong>
            <span>${latency}</span>
          </div>
          <div class="device-meta">${escapeHtml(device.ip)}:${device.port}</div>
        </button>
      `;
    })
    .join("");

  refs.deviceList.querySelectorAll("[data-device-id]").forEach((node) => {
    node.addEventListener("click", () => {
      state.selectedDeviceId = node.dataset.deviceId;
      renderDevices();
      updateSendButton();
    });
  });
}

function renderSelectedFiles() {
  if (state.selectedFiles.length === 0) {
    refs.selectedFiles.innerHTML = '<div class="subtle">No file selected.</div>';
    return;
  }

  refs.selectedFiles.innerHTML = state.selectedFiles
    .map((path) => {
      const name = fileNameFromPath(path);
      return `<div class="file-row"><span class="file-name">${escapeHtml(name)}</span><span class="file-size">${escapeHtml(path)}</span></div>`;
    })
    .join("");
}

function actionButtons(transfer) {
  const buttons = [];

  if (transfer.direction === "incoming" && transfer.status === "pending") {
    buttons.push(`<button class="btn btn-primary" data-action="accept" data-transfer-id="${transfer.id}">Accept</button>`);
    buttons.push(`<button class="btn btn-danger" data-action="reject" data-transfer-id="${transfer.id}">Reject</button>`);
  }

  if (transfer.direction === "outgoing" && transfer.status === "in_progress") {
    buttons.push(`<button class="btn btn-ghost" data-action="pause" data-transfer-id="${transfer.id}">Pause</button>`);
    buttons.push(`<button class="btn btn-danger" data-action="cancel" data-transfer-id="${transfer.id}">Cancel</button>`);
  }

  if (transfer.direction === "outgoing" && transfer.status === "paused") {
    buttons.push(`<button class="btn btn-primary" data-action="resume" data-transfer-id="${transfer.id}">Resume</button>`);
    buttons.push(`<button class="btn btn-danger" data-action="cancel" data-transfer-id="${transfer.id}">Cancel</button>`);
  }

  return buttons.join("");
}

function renderTransfers() {
  if (state.transfers.length === 0) {
    refs.transferList.innerHTML = '<div class="subtle">No transfer activity yet.</div>';
    return;
  }

  refs.transferList.innerHTML = state.transfers
    .map((transfer) => {
      const files = (transfer.files || []).map((f) => f.name).join(", ") || "-";
      const amount = `${bytesToText(transfer.transferredBytes)} / ${bytesToText(transfer.totalBytes)}`;
      const err = transfer.error ? `<div class="subtle">${escapeHtml(transfer.error)}</div>` : "";
      return `
        <div class="transfer-item">
          <div class="transfer-head">
            <div class="transfer-title">${escapeHtml(transfer.peerName || transfer.peerId || "Unknown device")}</div>
            <span class="badge ${transfer.status}">${escapeHtml(transfer.direction)} • ${escapeHtml(transfer.status)}</span>
          </div>
          <div class="subtle">${escapeHtml(files)}</div>
          <div class="progress-wrap"><div class="progress-bar" style="width: ${percent(transfer.progress)}"></div></div>
          <div class="transfer-meta">
            <span>${amount}</span>
            <span>${percent(transfer.progress)}</span>
          </div>
          ${err}
          <div class="transfer-actions">${actionButtons(transfer)}</div>
        </div>
      `;
    })
    .join("");

  refs.transferList.querySelectorAll("[data-action]").forEach((node) => {
    node.addEventListener("click", async () => {
      const action = node.dataset.action;
      const transferId = node.dataset.transferId;
      await handleTransferAction(action, transferId);
    });
  });
}

function escapeHtml(input) {
  return String(input || "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function updateSendButton() {
  refs.sendBtn.disabled = !(state.selectedDeviceId && state.selectedFiles.length > 0);
}

async function refreshDevices() {
  const api = appApi();
  if (!api) {
    return;
  }
  const devices = await api.ListDevices();
  state.devices = (Array.isArray(devices) ? devices : []).filter((d) => d && d.isOnline);

  if (state.selectedDeviceId) {
    const stillExists = state.devices.some((d) => d.id === state.selectedDeviceId);
    if (!stillExists) {
      state.selectedDeviceId = "";
    }
  }

  renderDevices();
  updateSendButton();
}

async function refreshTransfers() {
  const api = appApi();
  if (!api) {
    return;
  }
  state.transfers = await api.ListTransfers();
  renderTransfers();
}

async function handleTransferAction(action, transferId) {
  const api = appApi();
  if (!api) {
    return;
  }

  try {
    if (action === "accept") {
      await api.AcceptTransfer(transferId);
    }
    if (action === "reject") {
      await api.RejectTransfer(transferId);
    }
    if (action === "pause") {
      await api.PauseTransfer(transferId);
    }
    if (action === "resume") {
      await api.ResumeTransfer(transferId);
    }
    if (action === "cancel") {
      await api.CancelTransfer(transferId);
    }
    await refreshTransfers();
  } catch (err) {
    setStatus(`Action failed: ${err}`);
  }
}

async function bootstrap() {
  const api = appApi();
  if (!api) {
    setStatus("Open this UI via Wails runtime.");
    return;
  }

  refs.pickFilesBtn.addEventListener("click", async () => {
    try {
      const files = await api.PickFiles();
      state.selectedFiles = Array.isArray(files) ? files : [];
      renderSelectedFiles();
      updateSendButton();
    } catch (err) {
      setStatus(`Pick files failed: ${err}`);
    }
  });

  refs.sendBtn.addEventListener("click", async () => {
    if (!state.selectedDeviceId || state.selectedFiles.length === 0) {
      return;
    }
    try {
      setStatus("Starting transfer...");
      await api.SendFiles(state.selectedDeviceId, state.selectedFiles);
      state.selectedFiles = [];
      renderSelectedFiles();
      updateSendButton();
      await refreshTransfers();
      setStatus("Transfer queued");
    } catch (err) {
      setStatus(`Send failed: ${err}`);
    }
  });

  try {
    state.info = await api.AppInfo();
    refs.deviceLabel.textContent = `${state.info.deviceName} (${state.info.deviceId})`; 
  } catch (err) {
    refs.deviceLabel.textContent = "Unable to read app info";
  }

  renderSelectedFiles();
  updateSendButton();

  await refreshDevices();
  await refreshTransfers();
  setStatus("Ready");

  if (!state.pollingStarted) {
    state.pollingStarted = true;
    window.setInterval(refreshDevices, 1000);
    window.setInterval(refreshTransfers, 1000);
  }

  if (window.runtime && typeof window.runtime.EventsOn === "function") {
    window.runtime.EventsOn("state:updated", async () => {
      await refreshDevices();
      await refreshTransfers();
    });
  }
}

window.addEventListener("DOMContentLoaded", () => {
  bootstrap().catch((err) => {
    setStatus(`Startup failed: ${err}`);
  });
});
