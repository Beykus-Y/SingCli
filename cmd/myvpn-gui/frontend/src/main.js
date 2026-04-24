import './styles.css';

const app = document.querySelector('#app');

// === State ===
let currentPage = 'home';
const speedTestResults = {}; // { [serverId]: { loading, latencyMs, error } }
let speedTestAllRunning = false;
let speedTestAllStop = false;
const speedTestProgress = { current: 0, total: 0 };

const state = {
  servers: [],
  subscriptions: [],
  selectedServer: '',
  mode: 'proxy',
  status: 'Loading servers...',
  running: false,
  connecting: false,
  canConnect: false,
  canDisconnect: false,
  logs: [],
  proxySocks: '127.0.0.1:1080',
  proxyHTTP: '127.0.0.1:1081',
};

const subscriptionForm = {
  name: '',
  url: '',
  enabled: true,
  interval: '1440',
  busy: false,
};

let refreshingSubscriptionId = null;

// === Core helpers ===
function backend() {
  return window.go?.main?.App;
}

function escapeHTML(value) {
  return String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;');
}

function mergeState(next) {
  Object.assign(state, next || {});
  if (!state.mode) state.mode = 'proxy';
  render();
}

function statusTone() {
  const text = state.status.toLowerCase();
  if (state.running || text.includes('connected')) return 'connected';
  if (state.connecting || text.includes('connecting')) return 'connecting';
  if (text.includes('failed') || text.includes('cannot') || text.includes('not ready')) return 'error';
  return 'ready';
}

function formatBytes(value) {
  if (value === null || value === undefined) return 'unknown';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let next = Number(value);
  let index = 0;
  while (next >= 1024 && index < units.length - 1) {
    next /= 1024;
    index += 1;
  }
  return `${next >= 10 || index === 0 ? next.toFixed(0) : next.toFixed(1)} ${units[index]}`;
}

function formatDate(value) {
  if (!value) return 'unknown';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

// === Server grouping ===
function groupedServers() {
  const local = [];
  const bySubId = {};
  for (const server of state.servers) {
    if (server.subscriptionId) {
      if (!bySubId[server.subscriptionId]) bySubId[server.subscriptionId] = [];
      bySubId[server.subscriptionId].push(server);
    } else {
      local.push(server);
    }
  }
  return { local, bySubId };
}

// === Server select options (with optgroups) ===
function serverSelectOptions() {
  if (!state.servers.length) return '<option disabled>No servers</option>';
  const { local, bySubId } = groupedServers();
  let html = '';

  if (local.length) {
    html += `<optgroup label="Local">`;
    for (const s of local) {
      html += `<option value="${escapeHTML(s.name)}" ${s.name === state.selectedServer ? 'selected' : ''}>
        ${escapeHTML(s.name)} (${escapeHTML(s.type)})
      </option>`;
    }
    html += `</optgroup>`;
  }

  for (const sub of state.subscriptions) {
    const servers = bySubId[sub.id] || [];
    if (!servers.length) continue;
    html += `<optgroup label="${escapeHTML(sub.name)}">`;
    for (const s of servers) {
      html += `<option value="${escapeHTML(s.name)}" ${s.name === state.selectedServer ? 'selected' : ''}>
        ${escapeHTML(s.name)} (${escapeHTML(s.type)})
      </option>`;
    }
    html += `</optgroup>`;
  }

  // Servers from unknown subscriptions (not in state.subscriptions yet)
  for (const [subId, servers] of Object.entries(bySubId)) {
    if (state.subscriptions.find((s) => s.id == subId)) continue;
    html += `<optgroup label="Subscription #${escapeHTML(subId)}">`;
    for (const s of servers) {
      html += `<option value="${escapeHTML(s.name)}" ${s.name === state.selectedServer ? 'selected' : ''}>
        ${escapeHTML(s.name)} (${escapeHTML(s.type)})
      </option>`;
    }
    html += `</optgroup>`;
  }

  return html;
}

// === Subscription helpers ===
function subscriptionUsage(sub) {
  const used = sub.usedBytes ?? ((sub.uploadBytes ?? 0) + (sub.downloadBytes ?? 0));
  const total = sub.totalBytes;
  if (total !== null && total !== undefined) return `${formatBytes(used)} / ${formatBytes(total)}`;
  if (used > 0) return formatBytes(used);
  return 'unknown';
}

function subscriptionRows(disabled) {
  if (!state.subscriptions.length) {
    return '<div class="sub-empty muted">No subscriptions</div>';
  }
  return state.subscriptions.map((sub) => `
    <article class="sub-row">
      <div class="sub-main">
        <div class="sub-title">
          <strong>${escapeHTML(sub.name)}</strong>
          <span class="sub-status-badge ${sub.enabled ? 'enabled' : 'paused'}">${sub.enabled ? 'enabled' : 'paused'}</span>
        </div>
        <div class="sub-url">${escapeHTML(sub.url)}</div>
        ${sub.lastError ? `<div class="sub-error">${escapeHTML(sub.lastError)}</div>` : ''}
      </div>
      <div class="sub-stats">
        <span>Used <strong>${escapeHTML(subscriptionUsage(sub))}</strong></span>
        <span>Down <strong>${escapeHTML(formatBytes(sub.downloadBytes))}</strong></span>
        <span>Up <strong>${escapeHTML(formatBytes(sub.uploadBytes))}</strong></span>
        <span>Expires <strong>${escapeHTML(formatDate(sub.expireAt))}</strong></span>
        <span>Auto <strong>${escapeHTML(sub.autoUpdateIntervalMinutes)} min</strong></span>
        ${sub.profileUpdateIntervalMinutes ? `<span>Provider <strong>${escapeHTML(sub.profileUpdateIntervalMinutes)} min</strong></span>` : ''}
      </div>
      <div class="sub-actions">
        <label class="toggle">
          <input type="checkbox" data-sub-toggle="${sub.id}" ${sub.enabled ? 'checked' : ''} ${disabled ? 'disabled' : ''}>
          <span></span>
        </label>
        <button data-sub-refresh="${sub.id}" ${disabled || refreshingSubscriptionId === sub.id ? 'disabled' : ''}>
          ${refreshingSubscriptionId === sub.id ? 'Refreshing…' : 'Refresh'}
        </button>
        <button class="danger-button" data-sub-delete="${sub.id}" ${disabled ? 'disabled' : ''}>Delete</button>
      </div>
    </article>
  `).join('');
}

// === Speed test helpers ===
function speedTestDisplay(id) {
  const r = speedTestResults[id];
  if (!r) return { text: 'Test', cls: '' };
  if (r.loading) return { text: '…', cls: 'ping-loading' };
  if (r.error || r.latencyMs < 0) return { text: 'Fail', cls: 'ping-err', title: r.error };
  const cls = r.latencyMs < 600 ? 'ping-ok' : r.latencyMs < 1800 ? 'ping-mid' : 'ping-slow';
  return { text: `${r.latencyMs}ms`, cls };
}

async function runSpeedTest(id) {
  const api = backend();
  if (!api) return;
  speedTestResults[id] = { loading: true };
  render();
  try {
    const result = await api.SpeedTestServer(id);
    speedTestResults[id] = { loading: false, latencyMs: result.latencyMs, error: result.error || '' };
  } catch (err) {
    speedTestResults[id] = { loading: false, latencyMs: -1, error: String(err) };
  }
  render();
}

async function runSpeedTestAll() {
  if (speedTestAllRunning) {
    speedTestAllStop = true;
    return;
  }
  const api = backend();
  if (!api) return;
  speedTestAllRunning = true;
  speedTestAllStop = false;
  const servers = [...state.servers];
  speedTestProgress.current = 0;
  speedTestProgress.total = servers.length;
  render();
  for (const server of servers) {
    if (speedTestAllStop) break;
    speedTestProgress.current += 1;
    speedTestResults[server.id] = { loading: true };
    render();
    try {
      const result = await api.SpeedTestServer(server.id);
      speedTestResults[server.id] = { loading: false, latencyMs: result.latencyMs, error: result.error || '' };
    } catch (err) {
      speedTestResults[server.id] = { loading: false, latencyMs: -1, error: String(err) };
    }
  }
  speedTestAllRunning = false;
  speedTestAllStop = false;
  speedTestProgress.current = 0;
  speedTestProgress.total = 0;
  render();
}

// === Render: Topbar ===
function renderTopbar() {
  return `
    <header class="topbar">
      <div class="topbar-brand" style="--wails-draggable:drag">
        <h1>MGB VPN</h1>
        <p>Secure tunnel control</p>
      </div>
      <nav class="topbar-nav" style="--wails-draggable:no-drag">
        <button class="nav-tab ${currentPage === 'home' ? 'active' : ''}" data-nav="home">Home</button>
        <button class="nav-tab ${currentPage === 'servers' ? 'active' : ''}" data-nav="servers">Servers</button>
      </nav>
      <span class="status ${statusTone()}" style="--wails-draggable:no-drag">${escapeHTML(state.status)}</span>
    </header>
  `;
}

// === Render: Home page ===
function renderHomePage() {
  const disabled = state.running || state.connecting;
  const primaryLabel = disabled ? 'Disconnect' : 'Connect';
  const primaryDisabled = disabled ? !state.canDisconnect : !state.canConnect;
  const logs = state.logs.length
    ? state.logs.map((line) => `<div>${escapeHTML(line)}</div>`).join('')
    : '<div class="muted">No events yet</div>';

  return `
    <section class="panel control-panel">
      <label class="field">
        <span>Server</span>
        <select id="server" ${disabled || !state.servers.length ? 'disabled' : ''}>
          ${serverSelectOptions()}
        </select>
      </label>
      <div class="field">
        <span>Mode</span>
        <div class="segmented" role="group" aria-label="Connection mode">
          <button id="mode-proxy" class="${state.mode === 'proxy' ? 'active' : ''}" ${disabled ? 'disabled' : ''}>Proxy</button>
          <button id="mode-tun" class="${state.mode === 'tun' ? 'active' : ''}" ${disabled ? 'disabled' : ''}>TUN</button>
        </div>
      </div>
      <button id="primary" class="primary ${state.running ? 'danger' : ''}" ${primaryDisabled ? 'disabled' : ''}>
        ${primaryLabel}
      </button>
    </section>

    <section class="info-grid">
      <div class="metric">
        <span>SOCKS5</span>
        <strong>${escapeHTML(state.proxySocks)}</strong>
      </div>
      <div class="metric">
        <span>HTTP</span>
        <strong>${escapeHTML(state.proxyHTTP)}</strong>
      </div>
    </section>

    <section class="panel subscription-panel">
      <div class="subscription-header">
        <h2>Subscriptions</h2>
        <button id="reload-subscriptions" ${disabled ? 'disabled' : ''}>Reload</button>
      </div>
      <div class="subscription-form">
        <input id="subscription-name" class="sub-f-name" placeholder="Name" value="${escapeHTML(subscriptionForm.name)}" ${subscriptionForm.busy ? 'disabled' : ''}>
        <input id="subscription-url" class="sub-f-url" placeholder="Subscription URL" value="${escapeHTML(subscriptionForm.url)}" ${subscriptionForm.busy ? 'disabled' : ''}>
        <input id="subscription-interval" class="sub-f-interval" type="number" min="1" step="1" value="${escapeHTML(subscriptionForm.interval)}" ${subscriptionForm.busy ? 'disabled' : ''}>
        <label class="check">
          <input id="subscription-enabled" type="checkbox" ${subscriptionForm.enabled ? 'checked' : ''} ${subscriptionForm.busy ? 'disabled' : ''}>
          <span>Auto</span>
        </label>
        <button id="add-subscription" class="primary sub-f-add" ${subscriptionForm.busy ? 'disabled' : ''}>
          ${subscriptionForm.busy ? 'Adding…' : 'Add'}
        </button>
      </div>
      <div class="subscription-list">${subscriptionRows(disabled)}</div>
    </section>

    ${state.status === 'TUN is not ready' ? `
      <section class="notice">TUN mode requires wintun.dll next to mgb-gui.exe. Rebuild with scripts\\build-gui.bat or scripts\\build-gui.ps1.</section>
    ` : ''}

    <section class="panel log-panel">
      <div class="log-header">
        <h2>Event Log</h2>
        <button id="reload" ${disabled ? 'disabled' : ''}>Reload servers</button>
      </div>
      <div id="log" class="log">${logs}</div>
    </section>
  `;
}

// === Render: Server card ===
function renderServerCard(server, isLocal) {
  const isSelected = server.name === state.selectedServer;
  const { text: testText, cls: testCls, title: testTitle } = speedTestDisplay(server.id);
  const r = speedTestResults[server.id];

  return `
    <div class="server-card ${isSelected ? 'selected' : ''}">
      <span class="type-badge" data-type="${escapeHTML(server.type.toLowerCase())}">${escapeHTML(server.type.toUpperCase())}</span>
      <span class="server-name" title="${escapeHTML(server.name)}">${escapeHTML(server.name)}</span>
      <span class="server-address">${escapeHTML(server.address)}</span>
      <div class="server-actions">
        <button class="action-btn use-btn ${isSelected ? 'use-selected' : ''}" data-use-server="${escapeHTML(server.name)}">
          ${isSelected ? '✓ Active' : 'Use'}
        </button>
        <button class="action-btn test-btn ${testCls}" data-test-server="${server.id}"
          ${r?.loading || speedTestAllRunning ? 'disabled' : ''}
          ${testTitle ? `title="${escapeHTML(testTitle)}"` : ''}>
          ${testText}
        </button>
        ${isLocal ? `<button class="action-btn del-btn" data-delete-server="${server.id}" title="Delete server">✕</button>` : ''}
      </div>
    </div>
  `;
}

// === Render: Servers page ===
function renderServersPage() {
  const { local, bySubId } = groupedServers();
  const total = state.servers.length;

  const testAllLabel = speedTestAllRunning
    ? `Stop (${speedTestProgress.current}/${speedTestProgress.total})`
    : 'Test All';

  let html = `
    <div class="page-header">
      <h2>Servers <span class="count-pill">${total}</span></h2>
      <div class="page-header-actions">
        <button id="test-all-btn" class="${speedTestAllRunning ? 'test-all-running' : ''}">${testAllLabel}</button>
        <button id="reload-servers-page">Reload</button>
      </div>
    </div>
  `;

  // Local group
  html += `
    <div class="server-group">
      <div class="server-group-header">
        <span>Local</span>
        <span class="group-count">${local.length}</span>
      </div>
      ${local.length === 0
        ? '<div class="group-empty muted">No local servers.</div>'
        : local.map((s) => renderServerCard(s, true)).join('')}
    </div>
  `;

  // Subscription groups — follow state.subscriptions order
  for (const sub of state.subscriptions) {
    const servers = bySubId[sub.id] || [];
    html += `
      <div class="server-group">
        <div class="server-group-header">
          <span>${escapeHTML(sub.name)}</span>
          <span class="group-count">${servers.length}</span>
          <span class="sub-status-badge ${sub.enabled ? 'enabled' : 'paused'}">${sub.enabled ? 'enabled' : 'paused'}</span>
        </div>
        ${servers.length === 0
          ? '<div class="group-empty muted">No servers yet — refresh subscription.</div>'
          : servers.map((s) => renderServerCard(s, false)).join('')}
      </div>
    `;
  }

  // Unknown subscription groups (edge case)
  for (const [subId, servers] of Object.entries(bySubId)) {
    if (state.subscriptions.find((s) => s.id == subId)) continue;
    html += `
      <div class="server-group">
        <div class="server-group-header">
          <span>Subscription #${escapeHTML(subId)}</span>
          <span class="group-count">${servers.length}</span>
        </div>
        ${servers.map((s) => renderServerCard(s, false)).join('')}
      </div>
    `;
  }

  return html;
}

// === Main render ===
function render() {
  app.innerHTML = `
    <div class="shell">
      ${renderTopbar()}
      <div class="page-content">
        ${currentPage === 'home' ? renderHomePage() : renderServersPage()}
      </div>
    </div>
  `;
  bindControls();
  if (currentPage === 'home') {
    const logBox = document.querySelector('#log');
    if (logBox) logBox.scrollTop = logBox.scrollHeight;
  }
}

// === Bind controls ===
function bindControls() {
  // Navigation
  document.querySelectorAll('[data-nav]').forEach((btn) => {
    btn.addEventListener('click', () => {
      currentPage = btn.dataset.nav;
      render();
    });
  });

  if (currentPage === 'home') {
    bindHomeControls();
  } else {
    bindServersControls();
  }
}

function bindHomeControls() {
  document.querySelector('#server')?.addEventListener('change', (e) => {
    state.selectedServer = e.target.value;
    render();
  });

  document.querySelector('#mode-proxy')?.addEventListener('click', () => {
    state.mode = 'proxy';
    render();
  });

  document.querySelector('#mode-tun')?.addEventListener('click', () => {
    state.mode = 'tun';
    render();
  });

  document.querySelector('#primary')?.addEventListener('click', async () => {
    const api = backend();
    if (!api) return;
    const next = state.running || state.connecting
      ? await api.Disconnect()
      : await api.Connect(state.selectedServer, state.mode);
    mergeState(next);
  });

  document.querySelector('#reload')?.addEventListener('click', async () => {
    const api = backend();
    if (!api) return;
    mergeState(await api.ReloadServers());
  });

  document.querySelector('#reload-subscriptions')?.addEventListener('click', async () => {
    await loadSubscriptions();
  });

  document.querySelector('#subscription-name')?.addEventListener('input', (e) => {
    subscriptionForm.name = e.target.value;
  });

  document.querySelector('#subscription-url')?.addEventListener('input', (e) => {
    subscriptionForm.url = e.target.value;
  });

  document.querySelector('#subscription-interval')?.addEventListener('input', (e) => {
    subscriptionForm.interval = e.target.value;
  });

  document.querySelector('#subscription-enabled')?.addEventListener('change', (e) => {
    subscriptionForm.enabled = e.target.checked;
  });

  document.querySelector('#add-subscription')?.addEventListener('click', async () => {
    const api = backend();
    if (!api || !subscriptionForm.url.trim()) return;
    subscriptionForm.busy = true;
    render();
    try {
      const interval = Number.parseInt(subscriptionForm.interval, 10);
      await api.AddSubscription({
        name: subscriptionForm.name.trim() || subscriptionForm.url.trim(),
        url: subscriptionForm.url.trim(),
        enabled: subscriptionForm.enabled,
        autoUpdateIntervalMinutes: Number.isFinite(interval) && interval > 0 ? interval : 1440,
      });
      subscriptionForm.name = '';
      subscriptionForm.url = '';
      subscriptionForm.interval = '1440';
      subscriptionForm.enabled = true;
      await loadSubscriptions();
      mergeState(await api.GetState());
    } finally {
      subscriptionForm.busy = false;
      render();
    }
  });

  document.querySelectorAll('[data-sub-refresh]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const api = backend();
      if (!api) return;
      refreshingSubscriptionId = Number(btn.dataset.subRefresh);
      render();
      try {
        await api.RefreshSubscription(refreshingSubscriptionId);
      } catch (err) {
        console.error(err);
      } finally {
        refreshingSubscriptionId = null;
        await loadSubscriptions();
        mergeState(await api.GetState());
      }
    });
  });

  document.querySelectorAll('[data-sub-delete]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const api = backend();
      if (!api) return;
      await api.DeleteSubscription(Number(btn.dataset.subDelete));
      await loadSubscriptions();
      mergeState(await api.GetState());
    });
  });

  document.querySelectorAll('[data-sub-toggle]').forEach((checkbox) => {
    checkbox.addEventListener('change', async () => {
      const api = backend();
      if (!api) return;
      const id = Number(checkbox.dataset.subToggle);
      const sub = state.subscriptions.find((s) => s.id === id);
      if (!sub) return;
      await api.UpdateSubscription(id, {
        name: sub.name,
        url: sub.url,
        enabled: checkbox.checked,
        autoUpdateIntervalMinutes: sub.autoUpdateIntervalMinutes,
      });
      await loadSubscriptions();
    });
  });
}

function bindServersControls() {
  document.querySelector('#reload-servers-page')?.addEventListener('click', async () => {
    const api = backend();
    if (!api) return;
    mergeState(await api.ReloadServers());
  });

  document.querySelector('#test-all-btn')?.addEventListener('click', () => {
    runSpeedTestAll();
  });

  document.querySelectorAll('[data-use-server]').forEach((btn) => {
    btn.addEventListener('click', () => {
      state.selectedServer = btn.dataset.useServer;
      currentPage = 'home';
      render();
    });
  });

  document.querySelectorAll('[data-test-server]').forEach((btn) => {
    btn.addEventListener('click', () => {
      runSpeedTest(Number(btn.dataset.testServer));
    });
  });

  document.querySelectorAll('[data-delete-server]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const api = backend();
      if (!api) return;
      const id = Number(btn.dataset.deleteServer);
      delete speedTestResults[id];
      await api.DeleteServer(id);
      mergeState(await api.ReloadServers());
    });
  });
}

// === Data loading ===
async function loadServers() {
  const api = backend();
  if (!api) return;
  const servers = await api.GetServers();
  if (servers?.length) {
    state.servers = servers;
    if (!state.selectedServer) state.selectedServer = servers[0].name;
    render();
  }
}

async function loadSubscriptions() {
  const api = backend();
  if (!api) return;
  state.subscriptions = (await api.ListSubscriptions()) || [];
  render();
}

// === Init ===
async function init() {
  render();

  window.runtime?.EventsOn?.('state', (next) => mergeState(next));
  window.runtime?.EventsOn?.('log', (line) => {
    if (line && !state.logs.includes(line)) {
      state.logs = [...state.logs, line].slice(-300);
      render();
    }
  });

  const api = backend();
  if (api) {
    await loadServers();
    await loadSubscriptions();
    mergeState(await api.GetState());
  }
}

init();
