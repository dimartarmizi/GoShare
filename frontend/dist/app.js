const state = {
	info: null,
	devices: [],
	transfers: [],
	selectedDeviceId: "",
	selectedFiles: [],
	pollingStarted: false,
	transferActionLocks: new Set(),
	pendingRefreshTimer: null,
};

const refs = {
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

function deviceIconMarkup() {
	return `
		<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round" class="icon icon-tabler icons-tabler-outline icon-tabler-ai-gateway" aria-hidden="true" focusable="false">
			<path stroke="none" d="M0 0h24v24H0z" fill="none" />
			<path d="M4 6.5a2.5 2.5 0 1 0 5 0a2.5 2.5 0 1 0 -5 0" />
			<path d="M15 6.5a2.5 2.5 0 1 0 5 0a2.5 2.5 0 1 0 -5 0" />
			<path d="M15 17.5a2.5 2.5 0 1 0 5 0a2.5 2.5 0 1 0 -5 0" />
			<path d="M4 17.5a2.5 2.5 0 1 0 5 0a2.5 2.5 0 1 0 -5 0" />
			<path d="M8.5 15.5l7 -7" />
		</svg>
	`;
}

function emptyDeviceStateMarkup() {
	return `
		<div class="empty-state empty-state-devices">
			<div class="empty-state-icon" aria-hidden="true">
				<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round" class="icon icon-tabler icons-tabler-outline icon-tabler-network-off" aria-hidden="true" focusable="false">
					<path stroke="none" d="M0 0h24v24H0z" fill="none" />
					<path d="M6.528 6.536a6 6 0 0 0 7.942 7.933m2.247 -1.76a6 6 0 0 0 -8.427 -8.425" />
					<path d="M12 3c1.333 .333 2 2.333 2 6c0 .337 -.006 .66 -.017 .968m-.55 3.473c-.333 .884 -.81 1.403 -1.433 1.559" />
					<path d="M12 3c-.936 .234 -1.544 1.29 -1.822 3.167m-.16 3.838c.116 3.029 .776 4.695 1.982 4.995" />
					<path d="M6 9h3m4 0h5" />
					<path d="M3 20h7" />
					<path d="M14 20h7" />
					<path d="M10 20a2 2 0 1 0 4 0a2 2 0 0 0 -4 0" />
					<path d="M12 15v3" />
					<path d="M3 3l18 18" />
				</svg>
			</div>
			<div class="empty-state-text">
					No Online Devices Discovered Yet
			</div>
		</div>
	`;
}

function emptyTransferStateMarkup() {
	return `
		<div class="empty-state empty-state-transfers">
			<div class="empty-state-icon" aria-hidden="true">
				<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round" class="icon icon-tabler icons-tabler-outline icon-tabler-history" aria-hidden="true" focusable="false">
					<path stroke="none" d="M0 0h24v24H0z" fill="none" />
					<path d="M12 8l0 4l2 2" />
					<path d="M3.05 11a9 9 0 1 1 .5 4m-.5 5v-5h5" />
				</svg>
			</div>
			<div class="empty-state-text">
				No Transfer Activity Yet
			</div>
		</div>
	`;
}

function renderDevices() {
	refs.deviceCount.textContent = `${state.devices.length} online`;

	if (state.devices.length === 0) {
		refs.deviceList.innerHTML = emptyDeviceStateMarkup();
		return;
	}

	refs.deviceList.innerHTML = state.devices
		.map((device) => {
			const isSelected = state.selectedDeviceId === device.id;
			return `
			<button class="device-item ${isSelected ? "selected" : ""}" data-device-id="${device.id}">
				<span class="device-visual" aria-hidden="true">
					<span class="device-avatar-icon">${deviceIconMarkup()}</span>
				</span>
				<span class="device-copy">
					<strong class="device-name">${escapeHtml(device.name)}</strong>
					<span class="device-meta">${escapeHtml(device.ip)}:${device.port}</span>
				</span>
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
		refs.selectedFiles.innerHTML = '<div class="subtle">No File Selected.</div>';
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
		refs.transferList.innerHTML = emptyTransferStateMarkup();
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
					<div class="transfer-title">${escapeHtml(transfer.peerName || transfer.peerId || "Unknown Device")}</div>
					<div class="transfer-badges">
						<span class="badge badge-direction">${escapeHtml(transfer.directionLabel || "Unknown")}</span>
						<span class="badge badge-status ${transfer.status}">${escapeHtml(transfer.statusLabel || "Unknown")}</span>
					</div>
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

function transferActionKey(action, transferId) {
	return `${action}:${transferId}`;
}

function scheduleRefresh(delayMs = 120) {
	if (state.pendingRefreshTimer != null) {
		window.clearTimeout(state.pendingRefreshTimer);
	}
	state.pendingRefreshTimer = window.setTimeout(async () => {
		state.pendingRefreshTimer = null;
		await refreshDevices();
		await refreshTransfers();
	}, delayMs);
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

	const key = transferActionKey(action, transferId);
	if (state.transferActionLocks.has(key)) {
		return;
	}

	state.transferActionLocks.add(key);

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
		void err;
	} finally {
		state.transferActionLocks.delete(key);
	}
}

async function bootstrap() {
	const api = appApi();
	if (!api) {
		return;
	}

	refs.pickFilesBtn.addEventListener("click", async () => {
		try {
			const files = await api.PickFiles();
			state.selectedFiles = Array.isArray(files) ? files : [];
			renderSelectedFiles();
			updateSendButton();
		} catch (err) {
			void err;
		}
	});

	refs.sendBtn.addEventListener("click", async () => {
		if (!state.selectedDeviceId || state.selectedFiles.length === 0) {
			return;
		}
		try {
			await api.SendFiles(state.selectedDeviceId, state.selectedFiles);
			state.selectedFiles = [];
			renderSelectedFiles();
			updateSendButton();
			await refreshTransfers();
		} catch (err) {
			void err;
		}
	});

	try {
		state.info = await api.AppInfo();
		refs.deviceLabel.textContent = `${state.info.deviceName} (${state.info.deviceIP}:${state.info.devicePort})`;
	} catch (err) {
		refs.deviceLabel.textContent = "Unable To Read App Info";
	}

	renderSelectedFiles();
	updateSendButton();

	await refreshDevices();
	await refreshTransfers();

	if (!state.pollingStarted) {
		state.pollingStarted = true;
		window.setInterval(refreshDevices, 1000);
		window.setInterval(refreshTransfers, 1000);
	}

	if (window.runtime && typeof window.runtime.EventsOn === "function") {
		window.runtime.EventsOn("state:updated", async () => {
			scheduleRefresh(120);
		});
	}
}

window.addEventListener("DOMContentLoaded", () => {
	bootstrap().catch((err) => {
		void err;
	});
});
