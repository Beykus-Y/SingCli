import './styles.css';

const app = document.querySelector('#app');

const state = {
  servers: [],
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
  if (!state.mode) {
    state.mode = 'proxy';
  }
  render();
}

function statusTone() {
  const text = state.status.toLowerCase();
  if (state.running || text.includes('connected')) return 'connected';
  if (state.connecting || text.includes('connecting')) return 'connecting';
  if (text.includes('failed') || text.includes('cannot') || text.includes('not ready')) return 'error';
  return 'ready';
}

function render() {
  const disabled = state.running || state.connecting;
  const primaryLabel = state.running || state.connecting ? 'Disconnect' : 'Connect';
  const primaryDisabled = state.running || state.connecting ? !state.canDisconnect : !state.canConnect;
  const logs = state.logs.length
    ? state.logs.map((line) => `<div>${escapeHTML(line)}</div>`).join('')
    : '<div class="muted">No events yet</div>';

  app.innerHTML = `
    <section class="shell">
      <header class="topbar">
        <div>
          <h1>MGB VPN</h1>
          <p>Secure tunnel control</p>
        </div>
        <span class="status ${statusTone()}">${escapeHTML(state.status)}</span>
      </header>

      <section class="panel control-panel">
        <label class="field">
          <span>Server</span>
          <select id="server" ${disabled || state.servers.length === 0 ? 'disabled' : ''}>
            ${state.servers.map((server) => `
              <option value="${escapeHTML(server.name)}" ${server.name === state.selectedServer ? 'selected' : ''}>
                ${escapeHTML(server.name)} (${escapeHTML(server.type)})
              </option>
            `).join('')}
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
    </section>
  `;

  bindControls();
  const logBox = document.querySelector('#log');
  if (logBox) {
    logBox.scrollTop = logBox.scrollHeight;
  }
}

function bindControls() {
  document.querySelector('#server')?.addEventListener('change', (event) => {
    state.selectedServer = event.target.value;
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
}

async function init() {
  render();

  window.runtime?.EventsOn?.('state', (next) => {
    mergeState(next);
  });
  window.runtime?.EventsOn?.('log', (line) => {
    if (line && !state.logs.includes(line)) {
      state.logs = [...state.logs, line].slice(-300);
      render();
    }
  });

  const api = backend();
  if (api) {
    mergeState(await api.GetState());
  }
}

init();
