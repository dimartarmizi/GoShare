const statusPill = document.getElementById('statusPill');
const targetInput = document.getElementById('targetInput');
const fileInput = document.getElementById('fileInput');
const deviceList = document.getElementById('deviceList');
const transferList = document.getElementById('transferList');

const pickBtn = document.getElementById('pickBtn');
const discoverBtn = document.getElementById('discoverBtn');
const sendBtn = document.getElementById('sendBtn');

let latestDevices = [];
let selectedDeviceId = '';

function formatDeviceLabel(device) {
  return `${device.name} (${device.ip}:${device.port})`;
}

function updateSelectedTargetInput() {
  const selected = latestDevices.find((d) => d.id === selectedDeviceId);
  if (!selected) {
    targetInput.value = '';
    return;
  }
  targetInput.value = formatDeviceLabel(selected);
}

function backend() {
  return window.go?.main?.UIAPI;
}

function setStatus(text) {
  statusPill.textContent = text;
}

function renderDevices(devices) {
  latestDevices = devices || [];

  if (latestDevices.length === 0) {
    selectedDeviceId = '';
  } else {
    const stillExists = latestDevices.some((d) => d.id === selectedDeviceId);
    if (!stillExists) {
      selectedDeviceId = latestDevices[0].id;
    }
  }

  updateSelectedTargetInput();
  deviceList.innerHTML = '';
  if (!devices || devices.length === 0) {
    deviceList.innerHTML = '<li>Tidak ada device terdeteksi.</li>';
    return;
  }
  devices.forEach((d) => {
    const li = document.createElement('li');
    if (d.id === selectedDeviceId) {
      li.classList.add('selected');
    }
    li.innerHTML = `<strong>${d.name}</strong><div class="meta">${d.ip}:${d.port} • ID: ${d.id}</div>`;
    li.onclick = () => {
      selectedDeviceId = d.id;
      updateSelectedTargetInput();
      renderDevices(latestDevices);
      setStatus(`Target selected: ${d.name}`);
    };
    deviceList.appendChild(li);
  });
}

function renderTransfers(items) {
  transferList.innerHTML = '';
  if (!items || items.length === 0) {
    transferList.innerHTML = '<li>Belum ada transfer.</li>';
    return;
  }

  items.forEach((t) => {
    const pct = t.total > 0 ? Math.floor((t.transferred / t.total) * 100) : 0;
    const li = document.createElement('li');
    li.innerHTML = `<strong>${t.fileName}</strong> <span>(${t.status})</span><div class="meta">${pct}% - ${t.transferred}/${t.total} bytes</div>${t.lastError ? `<div class="meta">Error: ${t.lastError}</div>` : ''}`;
    transferList.appendChild(li);
  });
}

async function refreshTransfers() {
  const api = backend();
  if (!api?.ListTransfers) return;
  try {
    const tasks = await api.ListTransfers();
    renderTransfers(tasks);
  } catch (err) {
    console.error(err);
  }
}

async function discoverDevices(auto = false) {
  const api = backend();
  if (!api?.DiscoverDevices) return;

  if (!auto) {
    setStatus('Discovering...');
  }

  try {
    const devices = await api.DiscoverDevices(2500);
    renderDevices(devices);
    if (devices.length === 0) {
      setStatus('Tidak ada device aktif');
      return;
    }
    if (auto) {
      setStatus(`Auto-discover: ${devices.length} device`);
    } else {
      setStatus(`Found ${devices.length} device(s)`);
    }
  } catch (err) {
    if (!auto) {
      setStatus('Discover failed');
    }
    console.error(err);
  }
}

pickBtn.addEventListener('click', async () => {
  const api = backend();
  if (!api?.PickFile) return;
  const path = await api.PickFile();
  if (path) fileInput.value = path;
});

discoverBtn.addEventListener('click', async () => {
  await discoverDevices(false);
});

sendBtn.addEventListener('click', async () => {
  const api = backend();
  if (!api?.SendFile) return;

  const file = fileInput.value.trim();

  if (!selectedDeviceId) {
    await discoverDevices(true);
  }

  if (!selectedDeviceId || !file) {
    setStatus('Pilih file dan pastikan ada device yang terdeteksi');
    return;
  }

  setStatus('Starting transfer...');
  try {
    await api.SendFile(selectedDeviceId, file);
    setStatus('Transfer started');
    refreshTransfers();
  } catch (err) {
    setStatus('Transfer failed to start');
    console.error(err);
  }
});

(async function boot() {
  const api = backend();
  if (!api?.Ping) {
    setStatus('Backend belum siap');
    return;
  }
  const pong = await api.Ping();
  setStatus(pong);
  renderDevices([]);
  renderTransfers([]);
  discoverDevices(true);
  setInterval(() => discoverDevices(true), 4000);
  setInterval(refreshTransfers, 1000);
})();
