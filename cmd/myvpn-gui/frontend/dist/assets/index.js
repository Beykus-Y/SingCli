const app=document.querySelector("#app"),state={servers:[],selectedServer:"",mode:"proxy",status:"Loading servers...",running:!1,connecting:!1,canConnect:!1,canDisconnect:!1,logs:[],proxySocks:"127.0.0.1:1080",proxyHTTP:"127.0.0.1:1081"};function backend(){return window.go?.main?.App}function escapeHTML(e){return String(e??"").replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;").replaceAll("'","&#039;")}function mergeState(e){Object.assign(state,e||{}),state.mode||(state.mode="proxy"),render()}function statusTone(){const e=state.status.toLowerCase();return state.running||e.includes("connected")?"connected":state.connecting||e.includes("connecting")?"connecting":e.includes("failed")||e.includes("cannot")||e.includes("not ready")?"error":"ready"}function render(){const e=state.running||state.connecting,t=state.running||state.connecting?"Disconnect":"Connect",n=state.running||state.connecting?!state.canDisconnect:!state.canConnect,s=state.logs.length?state.logs.map(o=>`<div>${escapeHTML(o)}</div>`).join(""):'<div class="muted">No events yet</div>';app.innerHTML=`
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
          <select id="server" ${e||state.servers.length===0?"disabled":""}>
            ${state.servers.map(o=>`
              <option value="${escapeHTML(o.name)}" ${o.name===state.selectedServer?"selected":""}>
                ${escapeHTML(o.name)} (${escapeHTML(o.type)})
              </option>
            `).join("")}
          </select>
        </label>

        <div class="field">
          <span>Mode</span>
          <div class="segmented" role="group" aria-label="Connection mode">
            <button id="mode-proxy" class="${state.mode==="proxy"?"active":""}" ${e?"disabled":""}>Proxy</button>
            <button id="mode-tun" class="${state.mode==="tun"?"active":""}" ${e?"disabled":""}>TUN</button>
          </div>
        </div>

        <button id="primary" class="primary ${state.running?"danger":""}" ${n?"disabled":""}>
          ${t}
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

      ${state.status==="TUN is not ready"?`
        <section class="notice">TUN mode requires wintun.dll next to mgb-gui.exe. Rebuild with scripts\\build-gui.bat or scripts\\build-gui.ps1.</section>
      `:""}

      <section class="panel log-panel">
        <div class="log-header">
          <h2>Event Log</h2>
          <button id="reload" ${e?"disabled":""}>Reload servers</button>
        </div>
        <div id="log" class="log">${s}</div>
      </section>
    </section>
  `,bindControls();const i=document.querySelector("#log");i&&(i.scrollTop=i.scrollHeight)}function bindControls(){document.querySelector("#server")?.addEventListener("change",e=>{state.selectedServer=e.target.value,render()}),document.querySelector("#mode-proxy")?.addEventListener("click",()=>{state.mode="proxy",render()}),document.querySelector("#mode-tun")?.addEventListener("click",()=>{state.mode="tun",render()}),document.querySelector("#primary")?.addEventListener("click",async()=>{const e=backend();if(!e)return;const t=state.running||state.connecting?await e.Disconnect():await e.Connect(state.selectedServer,state.mode);mergeState(t)}),document.querySelector("#reload")?.addEventListener("click",async()=>{const e=backend();e&&mergeState(await e.ReloadServers())})}async function init(){render(),window.runtime?.EventsOn?.("state",e=>{mergeState(e)}),window.runtime?.EventsOn?.("log",e=>{e&&!state.logs.includes(e)&&(state.logs=[...state.logs,e].slice(-300),render())});const e=backend();e&&mergeState(await e.GetState())}init();
