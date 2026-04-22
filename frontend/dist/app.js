const state = {
	info: null,
	devices: [],
	transfers: [],
	selectedDeviceId: "",
	selectedFiles: [],
	pollingStarted: false,
	transferActionLocks: new Set(),
	pendingRefreshTimer: null,
	decisionModalAction: null,
	incomingPromptOpen: false,
	dragActive: false,
	filePreviewCache: new Map(),
	selectedFilesRenderToken: 0,
	transferDrawerOpen: false,
	transferUnreadIds: new Set(),
	transferKnownIds: new Set(),
	transferFabBumpTimer: null,
	transferInitialSyncDone: false,
};

const refs = {
	deviceLabel: document.getElementById("deviceLabel"),
	deviceCount: document.getElementById("deviceCount"),
	deviceList: document.getElementById("deviceList"),
	selectedFiles: document.getElementById("selectedFiles"),
	transferList: document.getElementById("transferList"),
	sendBtn: document.getElementById("sendBtn"),
	sendDropzone: document.getElementById("sendDropzone"),
	decisionModal: document.getElementById("decisionModal"),
	transfersDrawer: document.getElementById("transfersDrawer"),
	transferDrawerToggle: document.getElementById("transferDrawerToggle"),
	transferBackdrop: document.getElementById("transferBackdrop"),
	transferDrawerBadge: document.getElementById("transferDrawerBadge"),
	transferDrawerCloseBtn: document.getElementById("transferDrawerCloseBtn"),
	decisionModalTitle: document.getElementById("decisionModalTitle"),
	decisionModalBody: document.getElementById("decisionModalBody"),
	decisionModalConfirm: document.getElementById("decisionModalConfirm"),
	decisionModalCancel: document.getElementById("decisionModalCancel"),
	decisionModalClose: document.getElementById("decisionModalClose"),
};

function appApi() {
	return window.go && window.go.main && window.go.main.App ? window.go.main.App : null;
}

function fileNameFromPath(path) {
	return path.split(/[/\\]/).pop() || path;
}

function fileExtension(path) {
	const name = fileNameFromPath(path);
	const index = name.lastIndexOf(".");
	if (index <= 0) {
		return "";
	}
	return name.slice(index + 1).toLowerCase();
}

function isImageFile(path) {
	return ["png", "jpg", "jpeg", "gif", "webp", "bmp", "svg"].includes(fileExtension(path));
}

function fileTypeLabel(path) {
	const ext = fileExtension(path);
	return ext ? ext.toUpperCase() : "FILE";
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
	if (state.devices.length === 0) {
		refs.deviceCount.textContent = "";
		refs.deviceCount.hidden = true;
	} else {
		refs.deviceCount.hidden = false;
		refs.deviceCount.textContent = `${state.devices.length} online`;
	}

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

async function getFilePreview(path) {
	if (!isImageFile(path)) {
		return null;
	}

	if (state.filePreviewCache.has(path)) {
		return state.filePreviewCache.get(path);
	}

	const api = appApi();
	if (!api || typeof api.GetFilePreview !== "function") {
		return null;
	}

	try {
		const preview = await api.GetFilePreview(path);
		if (typeof preview === "string" && preview.startsWith("data:image/")) {
			state.filePreviewCache.set(path, preview);
			return preview;
		}
	} catch (err) {
		void err;
	}

	return null;
}

async function renderSelectedFiles() {
	const renderToken = state.selectedFilesRenderToken + 1;
	state.selectedFilesRenderToken = renderToken;
	refs.sendDropzone.classList.toggle("has-files", state.selectedFiles.length > 0);

	if (state.selectedFiles.length === 0) {
		refs.selectedFiles.innerHTML = "";
		return;
	}

	const cards = await Promise.all(
		state.selectedFiles.map(async (path) => {
			const name = fileNameFromPath(path);
			const previewUrl = await getFilePreview(path);

			if (previewUrl) {
				return `
					<div class="selected-file-card">
						<div class="selected-file-preview selected-file-preview-image">
							<button class="selected-file-remove" type="button" data-remove-file="${escapeHtml(path)}" aria-label="Remove ${escapeHtml(name)}">
								<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round" class="icon icon-tabler icons-tabler-outline icon-tabler-trash" aria-hidden="true" focusable="false"><path stroke="none" d="M0 0h24v24H0z" fill="none" /><path d="M4 7l16 0" /><path d="M10 11l0 6" /><path d="M14 11l0 6" /><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12" /><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3" /></svg>
							</button>
							<img src="${previewUrl}" alt="${escapeHtml(name)}" />
						</div>
						<div class="selected-file-meta">
							<span class="selected-file-name">${escapeHtml(name)}</span>
							<span class="selected-file-type">${escapeHtml(fileTypeLabel(path))}</span>
						</div>
					</div>
				`;
			}

			return `
				<div class="selected-file-card">
					<div class="selected-file-preview">
						<button class="selected-file-remove" type="button" data-remove-file="${escapeHtml(path)}" aria-label="Remove ${escapeHtml(name)}">
							<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round" class="icon icon-tabler icons-tabler-outline icon-tabler-trash" aria-hidden="true" focusable="false"><path stroke="none" d="M0 0h24v24H0z" fill="none" /><path d="M4 7l16 0" /><path d="M10 11l0 6" /><path d="M14 11l0 6" /><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12" /><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3" /></svg>
						</button>
						<div class="selected-file-icon" aria-hidden="true">
							<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round" class="icon icon-tabler icons-tabler-outline icon-tabler-file"><path stroke="none" d="M0 0h24v24H0z" fill="none" /><path d="M14 3v4a1 1 0 0 0 1 1h4" /><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2" /><path d="M9 17h6" /><path d="M9 13h6" /></svg>
						</div>
					</div>
					<div class="selected-file-meta">
						<span class="selected-file-name">${escapeHtml(name)}</span>
						<span class="selected-file-type">${escapeHtml(fileTypeLabel(path))}</span>
					</div>
				</div>
			`;
		})
	);

	if (state.selectedFilesRenderToken !== renderToken) {
		return;
	}

	refs.selectedFiles.innerHTML = cards.join("");

	refs.selectedFiles.querySelectorAll("[data-remove-file]").forEach((node) => {
		node.addEventListener("click", async (event) => {
			event.preventDefault();
			event.stopPropagation();

			const filePath = node.dataset.removeFile;
			if (!filePath) {
				return;
			}

			state.selectedFiles = state.selectedFiles.filter((item) => item !== filePath);
			await renderSelectedFiles();
			updateSendButton();
		});
	});
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

function updateTransferBadge() {
	const unreadCount = state.transferUnreadIds.size;
	if (!refs.transferDrawerBadge) {
		return;
	}

	if (unreadCount > 0) {
		refs.transferDrawerBadge.hidden = false;
		refs.transferDrawerBadge.textContent = unreadCount > 99 ? "99+" : String(unreadCount);
		return;
	}

	refs.transferDrawerBadge.hidden = true;
	refs.transferDrawerBadge.textContent = "";
}

function bumpTransferFab() {
	if (!refs.transferDrawerToggle) {
		return;
	}

	refs.transferDrawerToggle.classList.remove("bump");
	void refs.transferDrawerToggle.offsetWidth;
	refs.transferDrawerToggle.classList.add("bump");

	if (state.transferFabBumpTimer != null) {
		window.clearTimeout(state.transferFabBumpTimer);
	}

	state.transferFabBumpTimer = window.setTimeout(() => {
		refs.transferDrawerToggle.classList.remove("bump");
		state.transferFabBumpTimer = null;
	}, 450);
}

function syncTransferUnread(transfers) {
	const nextIds = new Set((transfers || []).map((transfer) => transfer.id));

	if (!state.transferInitialSyncDone) {
		state.transferKnownIds = nextIds;
		state.transferUnreadIds.clear();
		state.transferInitialSyncDone = true;
		updateTransferBadge();
		return 0;
	}

	state.transferUnreadIds = new Set(Array.from(state.transferUnreadIds).filter((transferId) => nextIds.has(transferId)));

	let newCount = 0;
	for (const transferId of nextIds) {
		if (!state.transferKnownIds.has(transferId) && !state.transferDrawerOpen) {
			state.transferUnreadIds.add(transferId);
			newCount += 1;
		}
	}

	state.transferKnownIds = nextIds;
	updateTransferBadge();

	if (newCount > 0) {
		bumpTransferFab();
	}

	return newCount;
}

function setTransferDrawerOpen(open) {
	state.transferDrawerOpen = open;
	refs.transfersDrawer.classList.toggle("open", open);
	refs.transferDrawerToggle.classList.toggle("drawer-open", open);
	if (refs.transferBackdrop) {
		refs.transferBackdrop.classList.toggle("open", open);
	}
	refs.transferDrawerToggle.setAttribute("aria-expanded", String(open));
	refs.transferDrawerToggle.setAttribute("aria-label", open ? "Close transfers" : "Open transfers");

	if (open) {
		state.transferUnreadIds.clear();
		updateTransferBadge();
	}
}

function transferActionKey(action, transferId) {
	return `${action}:${transferId}`;
}

function openDecisionModal(action, transferId) {
	const transfer = state.transfers.find((item) => item.id === transferId);
	if (!transfer) {
		return;
	}

	const isAccept = action === "accept";
	const fileNames = (transfer.files || []).map((file) => file.name).filter(Boolean);
	refs.decisionModalTitle.textContent = isAccept ? "Accept incoming transfer?" : "Reject incoming transfer?";
	refs.decisionModalBody.textContent = isAccept
		? `Receive ${fileNames.length > 0 ? `${fileNames.length} file(s)` : "the incoming files"} from ${transfer.peerName || transfer.peerId || "Unknown Device"}.`
		: `Reject ${fileNames.length > 0 ? `${fileNames.length} file(s)` : "this incoming transfer"} from ${transfer.peerName || transfer.peerId || "Unknown Device"}.`;
	refs.decisionModalConfirm.textContent = isAccept ? "Accept" : "Reject";
	refs.decisionModalConfirm.classList.toggle("btn-danger", !isAccept);
	refs.decisionModalConfirm.classList.toggle("btn-primary", isAccept);
	state.decisionModalAction = { action, transferId };
	state.incomingPromptOpen = true;
	refs.decisionModal.classList.add("open");
	refs.decisionModal.setAttribute("aria-hidden", "false");
	refs.decisionModalConfirm.focus();
}

function closeDecisionModal() {
	state.decisionModalAction = null;
	state.incomingPromptOpen = false;
	refs.decisionModal.classList.remove("open");
	refs.decisionModal.setAttribute("aria-hidden", "true");
}

async function confirmDecisionModal() {
	if (!state.decisionModalAction) {
		return;
	}

	const { action, transferId } = state.decisionModalAction;
	closeDecisionModal();
	await handleTransferAction(action, transferId, true);
}

function promptIncomingTransfer() {
	if (state.incomingPromptOpen || state.decisionModalAction) {
		return;
	}

	const incoming = state.transfers.find((transfer) => transfer.direction === "incoming" && transfer.status === "pending");
	if (!incoming) {
		return;
	}

	openDecisionModal("accept", incoming.id);
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

function setDropzoneActive(active) {
	state.dragActive = active;
	if (refs.sendDropzone) {
		refs.sendDropzone.classList.toggle("drag-active", active);
	}
}

function extractDroppedPaths(dataTransfer) {
	const paths = [];
	if (!dataTransfer) {
		return paths;
	}

	if (dataTransfer.files && dataTransfer.files.length > 0) {
		for (const file of Array.from(dataTransfer.files)) {
			const filePath = file.path || file.fullPath || file.webkitRelativePath || "";
			if (filePath) {
				paths.push(filePath);
			}
		}
	}

	if (paths.length === 0 && dataTransfer.items && dataTransfer.items.length > 0) {
		for (const item of Array.from(dataTransfer.items)) {
			if (item.kind !== "file") {
				continue;
			}
			const file = item.getAsFile();
			const filePath = file && (file.path || file.fullPath || file.webkitRelativePath || "");
			if (filePath) {
				paths.push(filePath);
			}
		}
	}

	return paths;
}

function mergeSelectedFiles(paths) {
	const nextPaths = Array.isArray(paths) ? paths : [];
	if (nextPaths.length === 0) {
		return false;
	}

	const existing = new Set(state.selectedFiles);
	let changed = false;
	for (const filePath of nextPaths) {
		if (!filePath || existing.has(filePath)) {
			continue;
		}
		existing.add(filePath);
		state.selectedFiles.push(filePath);
		changed = true;
	}

	return changed;
}

async function openFilePicker() {
	const api = appApi();
	if (!api) {
		return;
	}

	try {
		const files = await api.PickFiles();
		if (mergeSelectedFiles(files)) {
			await renderSelectedFiles();
			updateSendButton();
		}
	} catch (err) {
		void err;
	}
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
	syncTransferUnread(state.transfers);
	renderTransfers();
	promptIncomingTransfer();
}

async function handleTransferAction(action, transferId, force = false) {
	const api = appApi();
	if (!api) {
		return;
	}

	if (!force && (action === "accept" || action === "reject")) {
		openDecisionModal(action, transferId);
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

	refs.sendDropzone.addEventListener("click", async () => {
		await openFilePicker();
	});

	refs.sendDropzone.addEventListener("keydown", async (event) => {
		if (event.key === "Enter" || event.key === " ") {
			event.preventDefault();
			await openFilePicker();
		}
	});

	refs.sendDropzone.addEventListener("dragenter", (event) => {
		event.preventDefault();
		setDropzoneActive(true);
	});

	refs.sendDropzone.addEventListener("dragover", (event) => {
		event.preventDefault();
		event.dataTransfer.dropEffect = "copy";
		setDropzoneActive(true);
	});

	refs.sendDropzone.addEventListener("dragleave", (event) => {
		if (event.target === refs.sendDropzone) {
			setDropzoneActive(false);
		}
	});

	refs.sendDropzone.addEventListener("drop", async (event) => {
		event.preventDefault();
		setDropzoneActive(false);

		const droppedPaths = extractDroppedPaths(event.dataTransfer);
		if (droppedPaths.length === 0) {
			return;
		}

		if (mergeSelectedFiles(droppedPaths)) {
			await renderSelectedFiles();
			updateSendButton();
		}
	});

	refs.sendBtn.addEventListener("click", async () => {
		if (!state.selectedDeviceId || state.selectedFiles.length === 0) {
			return;
		}
		try {
			await api.SendFiles(state.selectedDeviceId, state.selectedFiles);
			state.selectedFiles = [];
			await renderSelectedFiles();
			updateSendButton();
			await refreshTransfers();
		} catch (err) {
			void err;
		}
	});

	refs.transferDrawerToggle.addEventListener("click", () => {
		setTransferDrawerOpen(!state.transferDrawerOpen);
	});

	refs.transferDrawerCloseBtn.addEventListener("click", () => {
		setTransferDrawerOpen(false);
	});

	if (refs.transferBackdrop) {
		refs.transferBackdrop.addEventListener("click", () => {
			setTransferDrawerOpen(false);
		});
	}

	refs.decisionModal.addEventListener("click", (event) => {
		if (event.target === refs.decisionModal) {
			if (state.decisionModalAction) {
				const { transferId } = state.decisionModalAction;
				closeDecisionModal();
				void handleTransferAction("reject", transferId, true);
			}
		}
	});

	refs.decisionModalCancel.addEventListener("click", async () => {
		if (state.decisionModalAction) {
			const { transferId } = state.decisionModalAction;
			closeDecisionModal();
			try {
				await handleTransferAction("reject", transferId, true);
			} catch (err) {
				void err;
			}
		}
	});
	refs.decisionModalClose.addEventListener("click", async () => {
		if (state.decisionModalAction) {
			const { transferId } = state.decisionModalAction;
			closeDecisionModal();
			try {
				await handleTransferAction("reject", transferId, true);
			} catch (err) {
				void err;
			}
		}
	});
	refs.decisionModalConfirm.addEventListener("click", async () => {
		try {
			await confirmDecisionModal();
		} catch (err) {
			void err;
		}
	});

	window.addEventListener("keydown", (event) => {
		if (event.key === "Escape" && refs.decisionModal.classList.contains("open") && state.decisionModalAction) {
			const { transferId } = state.decisionModalAction;
			closeDecisionModal();
			void handleTransferAction("reject", transferId, true);
			return;
		}
		if (event.key === "Escape" && state.transferDrawerOpen) {
			setTransferDrawerOpen(false);
		}
	});

	window.addEventListener("dragend", () => {
		setDropzoneActive(false);
	});

	try {
		state.info = await api.AppInfo();
		refs.deviceLabel.textContent = `${state.info.deviceName} (${state.info.deviceIP}:${state.info.devicePort})`;
	} catch (err) {
		refs.deviceLabel.textContent = "Unable To Read App Info";
	}

	await renderSelectedFiles();
	updateSendButton();

	await refreshDevices();
	await refreshTransfers();
	setTransferDrawerOpen(false);

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
