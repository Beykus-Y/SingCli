(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const i of document.querySelectorAll('link[rel="modulepreload"]'))a(i);new MutationObserver(i=>{for(const r of i)if(r.type==="childList")for(const u of r.addedNodes)u.tagName==="LINK"&&u.rel==="modulepreload"&&a(u)}).observe(document,{childList:!0,subtree:!0});function n(i){const r={};return i.integrity&&(r.integrity=i.integrity),i.referrerPolicy&&(r.referrerPolicy=i.referrerPolicy),i.crossOrigin==="use-credentials"?r.credentials="include":i.crossOrigin==="anonymous"?r.credentials="omit":r.credentials="same-origin",r}function a(i){if(i.ep)return;i.ep=!0;const r=n(i);fetch(i.href,r)}})();const I=document.querySelector("#app");let y="home";const g={};let S=!1,T=!1;const m={current:0,total:0},s={servers:[],subscriptions:[],selectedServer:"",mode:"proxy",status:"Loading servers...",running:!1,connecting:!1,canConnect:!1,canDisconnect:!1,logs:[],proxySocks:"127.0.0.1:1080",proxyHTTP:"127.0.0.1:1081"},c={name:"",url:"",enabled:!0,interval:"1440",busy:!1};let h=null;function v(){var e,t;return(t=(e=window.go)==null?void 0:e.main)==null?void 0:t.App}function o(e){return String(e??"").replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;").replaceAll("'","&#039;")}function b(e){Object.assign(s,e||{}),s.mode||(s.mode="proxy"),d()}function U(){const e=s.status.toLowerCase();return s.running||e.includes("connected")?"connected":s.connecting||e.includes("connecting")?"connecting":e.includes("failed")||e.includes("cannot")||e.includes("not ready")?"error":"ready"}function w(e){if(e==null)return"unknown";const t=["B","KB","MB","GB","TB"];let n=Number(e),a=0;for(;n>=1024&&a<t.length-1;)n/=1024,a+=1;return`${n>=10||a===0?n.toFixed(0):n.toFixed(1)} ${t[a]}`}function R(e){if(!e)return"unknown";const t=new Date(e);return Number.isNaN(t.getTime())?e:t.toLocaleString()}function k(){const e=[],t={};for(const n of s.servers)n.subscriptionId?(t[n.subscriptionId]||(t[n.subscriptionId]=[]),t[n.subscriptionId].push(n)):e.push(n);return{local:e,bySubId:t}}function B(){if(!s.servers.length)return"<option disabled>No servers</option>";const{local:e,bySubId:t}=k();let n="";if(e.length){n+='<optgroup label="Local">';for(const a of e)n+=`<option value="${o(a.name)}" ${a.name===s.selectedServer?"selected":""}>
        ${o(a.name)} (${o(a.type)})
      </option>`;n+="</optgroup>"}for(const a of s.subscriptions){const i=t[a.id]||[];if(i.length){n+=`<optgroup label="${o(a.name)}">`;for(const r of i)n+=`<option value="${o(r.name)}" ${r.name===s.selectedServer?"selected":""}>
        ${o(r.name)} (${o(r.type)})
      </option>`;n+="</optgroup>"}}for(const[a,i]of Object.entries(t))if(!s.subscriptions.find(r=>r.id==a)){n+=`<optgroup label="Subscription #${o(a)}">`;for(const r of i)n+=`<option value="${o(r.name)}" ${r.name===s.selectedServer?"selected":""}>
        ${o(r.name)} (${o(r.type)})
      </option>`;n+="</optgroup>"}return n}function C(e){const t=e.usedBytes??(e.uploadBytes??0)+(e.downloadBytes??0),n=e.totalBytes;return n!=null?`${w(t)} / ${w(n)}`:t>0?w(t):"unknown"}function D(e){return s.subscriptions.length?s.subscriptions.map(t=>`
    <article class="sub-row">
      <div class="sub-main">
        <div class="sub-title">
          <strong>${o(t.name)}</strong>
          <span class="sub-status-badge ${t.enabled?"enabled":"paused"}">${t.enabled?"enabled":"paused"}</span>
        </div>
        <div class="sub-url">${o(t.url)}</div>
        ${t.lastError?`<div class="sub-error">${o(t.lastError)}</div>`:""}
      </div>
      <div class="sub-stats">
        <span>Used <strong>${o(C(t))}</strong></span>
        <span>Down <strong>${o(w(t.downloadBytes))}</strong></span>
        <span>Up <strong>${o(w(t.uploadBytes))}</strong></span>
        <span>Expires <strong>${o(R(t.expireAt))}</strong></span>
        <span>Auto <strong>${o(t.autoUpdateIntervalMinutes)} min</strong></span>
        ${t.profileUpdateIntervalMinutes?`<span>Provider <strong>${o(t.profileUpdateIntervalMinutes)} min</strong></span>`:""}
      </div>
      <div class="sub-actions">
        <label class="toggle">
          <input type="checkbox" data-sub-toggle="${t.id}" ${t.enabled?"checked":""} ${e?"disabled":""}>
          <span></span>
        </label>
        <button data-sub-refresh="${t.id}" ${e||h===t.id?"disabled":""}>
          ${h===t.id?"Refreshing…":"Refresh"}
        </button>
        <button class="danger-button" data-sub-delete="${t.id}" ${e?"disabled":""}>Delete</button>
      </div>
    </article>
  `).join(""):'<div class="sub-empty muted">No subscriptions</div>'}function P(e){const t=g[e];if(!t)return{text:"Test",cls:""};if(t.loading)return{text:"…",cls:"ping-loading"};if(t.error||t.latencyMs<0)return{text:"Fail",cls:"ping-err",title:t.error};const n=t.latencyMs<600?"ping-ok":t.latencyMs<1800?"ping-mid":"ping-slow";return{text:`${t.latencyMs}ms`,cls:n}}async function O(e){const t=v();if(t){g[e]={loading:!0},d();try{const n=await t.SpeedTestServer(e);g[e]={loading:!1,latencyMs:n.latencyMs,error:n.error||""}}catch(n){g[e]={loading:!1,latencyMs:-1,error:String(n)}}d()}}async function H(){if(S){T=!0;return}const e=v();if(!e)return;S=!0,T=!1;const t=[...s.servers];m.current=0,m.total=t.length,d();for(const n of t){if(T)break;m.current+=1,g[n.id]={loading:!0},d();try{const a=await e.SpeedTestServer(n.id);g[n.id]={loading:!1,latencyMs:a.latencyMs,error:a.error||""}}catch(a){g[n.id]={loading:!1,latencyMs:-1,error:String(a)}}}S=!1,T=!1,m.current=0,m.total=0,d()}function j(){return`
    <header class="topbar">
      <div class="topbar-brand" style="--wails-draggable:drag">
        <h1>MGB VPN</h1>
        <p>Secure tunnel control</p>
      </div>
      <nav class="topbar-nav" style="--wails-draggable:no-drag">
        <button class="nav-tab ${y==="home"?"active":""}" data-nav="home">Home</button>
        <button class="nav-tab ${y==="servers"?"active":""}" data-nav="servers">Servers</button>
      </nav>
      <span class="status ${U()}" style="--wails-draggable:no-drag">${o(s.status)}</span>
    </header>
  `}function G(){const e=s.running||s.connecting,t=e?"Disconnect":"Connect",n=e?!s.canDisconnect:!s.canConnect,a=s.logs.length?s.logs.map(i=>`<div>${o(i)}</div>`).join(""):'<div class="muted">No events yet</div>';return`
    <section class="panel control-panel">
      <label class="field">
        <span>Server</span>
        <select id="server" ${e||!s.servers.length?"disabled":""}>
          ${B()}
        </select>
      </label>
      <div class="field">
        <span>Mode</span>
        <div class="segmented" role="group" aria-label="Connection mode">
          <button id="mode-proxy" class="${s.mode==="proxy"?"active":""}" ${e?"disabled":""}>Proxy</button>
          <button id="mode-tun" class="${s.mode==="tun"?"active":""}" ${e?"disabled":""}>TUN</button>
        </div>
      </div>
      <button id="primary" class="primary ${s.running?"danger":""}" ${n?"disabled":""}>
        ${t}
      </button>
    </section>

    <section class="info-grid">
      <div class="metric">
        <span>SOCKS5</span>
        <strong>${o(s.proxySocks)}</strong>
      </div>
      <div class="metric">
        <span>HTTP</span>
        <strong>${o(s.proxyHTTP)}</strong>
      </div>
    </section>

    <section class="panel subscription-panel">
      <div class="subscription-header">
        <h2>Subscriptions</h2>
        <button id="reload-subscriptions" ${e?"disabled":""}>Reload</button>
      </div>
      <div class="subscription-form">
        <input id="subscription-name" class="sub-f-name" placeholder="Name" value="${o(c.name)}" ${c.busy?"disabled":""}>
        <input id="subscription-url" class="sub-f-url" placeholder="Subscription URL" value="${o(c.url)}" ${c.busy?"disabled":""}>
        <input id="subscription-interval" class="sub-f-interval" type="number" min="1" step="1" value="${o(c.interval)}" ${c.busy?"disabled":""}>
        <label class="check">
          <input id="subscription-enabled" type="checkbox" ${c.enabled?"checked":""} ${c.busy?"disabled":""}>
          <span>Auto</span>
        </label>
        <button id="add-subscription" class="primary sub-f-add" ${c.busy?"disabled":""}>
          ${c.busy?"Adding…":"Add"}
        </button>
      </div>
      <div class="subscription-list">${D(e)}</div>
    </section>

    ${s.status==="TUN is not ready"?`
      <section class="notice">TUN mode requires wintun.dll next to mgb-gui.exe. Rebuild with scripts\\build-gui.bat or scripts\\build-gui.ps1.</section>
    `:""}

    <section class="panel log-panel">
      <div class="log-header">
        <h2>Event Log</h2>
        <button id="reload" ${e?"disabled":""}>Reload servers</button>
      </div>
      <div id="log" class="log">${a}</div>
    </section>
  `}function x(e,t){const n=e.name===s.selectedServer,{text:a,cls:i,title:r}=P(e.id),u=g[e.id];return`
    <div class="server-card ${n?"selected":""}">
      <span class="type-badge" data-type="${o(e.type.toLowerCase())}">${o(e.type.toUpperCase())}</span>
      <span class="server-name" title="${o(e.name)}">${o(e.name)}</span>
      <span class="server-address">${o(e.address)}</span>
      <div class="server-actions">
        <button class="action-btn use-btn ${n?"use-selected":""}" data-use-server="${o(e.name)}">
          ${n?"✓ Active":"Use"}
        </button>
        <button class="action-btn test-btn ${i}" data-test-server="${e.id}"
          ${u!=null&&u.loading||S?"disabled":""}
          ${r?`title="${o(r)}"`:""}>
          ${a}
        </button>
        ${t?`<button class="action-btn del-btn" data-delete-server="${e.id}" title="Delete server">✕</button>`:""}
      </div>
    </div>
  `}function F(){const{local:e,bySubId:t}=k(),n=s.servers.length,a=S?`Stop (${m.current}/${m.total})`:"Test All";let i=`
    <div class="page-header">
      <h2>Servers <span class="count-pill">${n}</span></h2>
      <div class="page-header-actions">
        <button id="test-all-btn" class="${S?"test-all-running":""}">${a}</button>
        <button id="reload-servers-page">Reload</button>
      </div>
    </div>
  `;i+=`
    <div class="server-group">
      <div class="server-group-header">
        <span>Local</span>
        <span class="group-count">${e.length}</span>
      </div>
      ${e.length===0?'<div class="group-empty muted">No local servers.</div>':e.map(r=>x(r,!0)).join("")}
    </div>
  `;for(const r of s.subscriptions){const u=t[r.id]||[];i+=`
      <div class="server-group">
        <div class="server-group-header">
          <span>${o(r.name)}</span>
          <span class="group-count">${u.length}</span>
          <span class="sub-status-badge ${r.enabled?"enabled":"paused"}">${r.enabled?"enabled":"paused"}</span>
        </div>
        ${u.length===0?'<div class="group-empty muted">No servers yet — refresh subscription.</div>':u.map(f=>x(f,!1)).join("")}
      </div>
    `}for(const[r,u]of Object.entries(t))s.subscriptions.find(f=>f.id==r)||(i+=`
      <div class="server-group">
        <div class="server-group-header">
          <span>Subscription #${o(r)}</span>
          <span class="group-count">${u.length}</span>
        </div>
        ${u.map(f=>x(f,!1)).join("")}
      </div>
    `);return i}function d(){if(I.innerHTML=`
    <div class="shell">
      ${j()}
      <div class="page-content">
        ${y==="home"?G():F()}
      </div>
    </div>
  `,K(),y==="home"){const e=document.querySelector("#log");e&&(e.scrollTop=e.scrollHeight)}}function K(){document.querySelectorAll("[data-nav]").forEach(e=>{e.addEventListener("click",()=>{y=e.dataset.nav,d()})}),y==="home"?V():z()}function V(){var e,t,n,a,i,r,u,f,A,q,N;(e=document.querySelector("#server"))==null||e.addEventListener("change",l=>{s.selectedServer=l.target.value,d()}),(t=document.querySelector("#mode-proxy"))==null||t.addEventListener("click",()=>{s.mode="proxy",d()}),(n=document.querySelector("#mode-tun"))==null||n.addEventListener("click",()=>{s.mode="tun",d()}),(a=document.querySelector("#primary"))==null||a.addEventListener("click",async()=>{const l=v();if(!l)return;const p=s.running||s.connecting?await l.Disconnect():await l.Connect(s.selectedServer,s.mode);b(p)}),(i=document.querySelector("#reload"))==null||i.addEventListener("click",async()=>{const l=v();l&&b(await l.ReloadServers())}),(r=document.querySelector("#reload-subscriptions"))==null||r.addEventListener("click",async()=>{await $()}),(u=document.querySelector("#subscription-name"))==null||u.addEventListener("input",l=>{c.name=l.target.value}),(f=document.querySelector("#subscription-url"))==null||f.addEventListener("input",l=>{c.url=l.target.value}),(A=document.querySelector("#subscription-interval"))==null||A.addEventListener("input",l=>{c.interval=l.target.value}),(q=document.querySelector("#subscription-enabled"))==null||q.addEventListener("change",l=>{c.enabled=l.target.checked}),(N=document.querySelector("#add-subscription"))==null||N.addEventListener("click",async()=>{const l=v();if(!(!l||!c.url.trim())){c.busy=!0,d();try{const p=Number.parseInt(c.interval,10);await l.AddSubscription({name:c.name.trim()||c.url.trim(),url:c.url.trim(),enabled:c.enabled,autoUpdateIntervalMinutes:Number.isFinite(p)&&p>0?p:1440}),c.name="",c.url="",c.interval="1440",c.enabled=!0,await $(),b(await l.GetState())}finally{c.busy=!1,d()}}}),document.querySelectorAll("[data-sub-refresh]").forEach(l=>{l.addEventListener("click",async()=>{const p=v();if(p){h=Number(l.dataset.subRefresh),d();try{await p.RefreshSubscription(h)}catch(L){console.error(L)}finally{h=null,await $(),b(await p.GetState())}}})}),document.querySelectorAll("[data-sub-delete]").forEach(l=>{l.addEventListener("click",async()=>{const p=v();p&&(await p.DeleteSubscription(Number(l.dataset.subDelete)),await $(),b(await p.GetState()))})}),document.querySelectorAll("[data-sub-toggle]").forEach(l=>{l.addEventListener("change",async()=>{const p=v();if(!p)return;const L=Number(l.dataset.subToggle),E=s.subscriptions.find(M=>M.id===L);E&&(await p.UpdateSubscription(L,{name:E.name,url:E.url,enabled:l.checked,autoUpdateIntervalMinutes:E.autoUpdateIntervalMinutes}),await $())})})}function z(){var e,t;(e=document.querySelector("#reload-servers-page"))==null||e.addEventListener("click",async()=>{const n=v();n&&b(await n.ReloadServers())}),(t=document.querySelector("#test-all-btn"))==null||t.addEventListener("click",()=>{H()}),document.querySelectorAll("[data-use-server]").forEach(n=>{n.addEventListener("click",()=>{s.selectedServer=n.dataset.useServer,y="home",d()})}),document.querySelectorAll("[data-test-server]").forEach(n=>{n.addEventListener("click",()=>{O(Number(n.dataset.testServer))})}),document.querySelectorAll("[data-delete-server]").forEach(n=>{n.addEventListener("click",async()=>{const a=v();if(!a)return;const i=Number(n.dataset.deleteServer);delete g[i],await a.DeleteServer(i),b(await a.ReloadServers())})})}async function J(){const e=v();if(!e)return;const t=await e.GetServers();t!=null&&t.length&&(s.servers=t,s.selectedServer||(s.selectedServer=t[0].name),d())}async function $(){const e=v();e&&(s.subscriptions=await e.ListSubscriptions()||[],d())}async function Q(){var t,n,a,i;d(),(n=(t=window.runtime)==null?void 0:t.EventsOn)==null||n.call(t,"state",r=>b(r)),(i=(a=window.runtime)==null?void 0:a.EventsOn)==null||i.call(a,"log",r=>{r&&!s.logs.includes(r)&&(s.logs=[...s.logs,r].slice(-300),d())});const e=v();e&&(await J(),await $(),b(await e.GetState()))}Q();
