(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const i of document.querySelectorAll('link[rel="modulepreload"]'))n(i);new MutationObserver(i=>{for(const a of i)if(a.type==="childList")for(const l of a.addedNodes)l.tagName==="LINK"&&l.rel==="modulepreload"&&n(l)}).observe(document,{childList:!0,subtree:!0});function s(i){const a={};return i.integrity&&(a.integrity=i.integrity),i.referrerPolicy&&(a.referrerPolicy=i.referrerPolicy),i.crossOrigin==="use-credentials"?a.credentials="include":i.crossOrigin==="anonymous"?a.credentials="omit":a.credentials="same-origin",a}function n(i){if(i.ep)return;i.ep=!0;const a=s(i);fetch(i.href,a)}})();const j=document.querySelector("#app");let T="home";const g={};let k=!1,q=!1;const w={current:0,total:0};let P="",C=!1,N="",R=null;const r={servers:[],subscriptions:[],selectedServer:"",mode:"proxy",status:"Loading servers...",running:!1,connecting:!1,canConnect:!1,canDisconnect:!1,logs:[],proxySocks:"127.0.0.1:1080",proxyHTTP:"127.0.0.1:1081",connectedAt:"",traffic:{sessionDownloadBytes:0,sessionUploadBytes:0,downloadBytesPerSec:0,uploadBytesPerSec:0}},u={name:"",url:"",enabled:!0,interval:"1440",busy:!1};let M=null;const v={source:"",busy:!1,message:"",error:""};function m(){var e,t;return(t=(e=window.go)==null?void 0:e.main)==null?void 0:t.App}function o(e){return String(e??"").replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;").replaceAll("'","&#039;")}function y(e){Object.assign(r,e||{}),r.mode||(r.mode="proxy"),r.traffic||(r.traffic={sessionDownloadBytes:0,sessionUploadBytes:0,downloadBytesPerSec:0,uploadBytesPerSec:0}),p()}function K(){const e=r.status.toLowerCase();return r.running||e.includes("connected")?"connected":r.connecting||e.includes("connecting")?"connecting":e.includes("failed")||e.includes("cannot")||e.includes("not ready")?"error":"ready"}function L(e){if(e==null)return"unknown";const t=["B","KB","MB","GB","TB"];let s=Number(e),n=0;for(;s>=1024&&n<t.length-1;)s/=1024,n+=1;return`${s>=10||n===0?s.toFixed(0):s.toFixed(1)} ${t[n]}`}function _(e){if(!e)return"unknown";const t=new Date(e);return Number.isNaN(t.getTime())?e:t.toLocaleString()}function V(e){if(!e)return"";const t=new Date(e);if(Number.isNaN(t.getTime()))return"";const s=Math.max(0,Math.floor((Date.now()-t.getTime())/6e4)),n=Math.floor(s/60),i=s%60;return n>0?`${n}h ${i}m`:`${i}m`}function U(e){const t=Number(e||0)/1024/1024;return`${t>=10?t.toFixed(1):t.toFixed(2)} MB/s`}function F(){var e;return((e=r.servers.map(t=>({server:t,result:g[t.id]})).filter(({result:t})=>t&&!t.loading&&!t.error&&t.latencyMs>=0).sort((t,s)=>t.result.latencyMs-s.result.latencyMs)[0])==null?void 0:e.server)||null}function H(){const e=[],t={};for(const s of r.servers)s.subscriptionId?(t[s.subscriptionId]||(t[s.subscriptionId]=[]),t[s.subscriptionId].push(s)):e.push(s);return{local:e,bySubId:t}}function G(e){const t=P.trim().toLowerCase();return t?`${e.name} ${e.address}`.toLowerCase().includes(t):!0}function B(e){const t=e.filter(G);return C?t.map((s,n)=>({server:s,index:n,result:g[s.id]})).sort((s,n)=>{const i=O(s.result),a=O(n.result);return i.bucket!==a.bucket?i.bucket-a.bucket:i.latency!==a.latency?i.latency-a.latency:s.index-n.index}).map(s=>s.server):t}function O(e){return e&&!e.loading&&!e.error&&e.latencyMs>=0?{bucket:0,latency:e.latencyMs}:e&&(e.error||e.latencyMs<0)?{bucket:2,latency:Number.MAX_SAFE_INTEGER}:{bucket:1,latency:Number.MAX_SAFE_INTEGER}}function X(){if(!r.servers.length)return"<option disabled>No servers</option>";const{local:e,bySubId:t}=H();let s="";if(e.length){s+='<optgroup label="Local">';for(const n of e)s+=`<option value="${o(n.name)}" ${n.name===r.selectedServer?"selected":""}>
        ${o(n.name)} (${o(n.type)})
      </option>`;s+="</optgroup>"}for(const n of r.subscriptions){const i=t[n.id]||[];if(i.length){s+=`<optgroup label="${o(n.name)}">`;for(const a of i)s+=`<option value="${o(a.name)}" ${a.name===r.selectedServer?"selected":""}>
        ${o(a.name)} (${o(a.type)})
      </option>`;s+="</optgroup>"}}for(const[n,i]of Object.entries(t))if(!r.subscriptions.find(a=>a.id==n)){s+=`<optgroup label="Subscription #${o(n)}">`;for(const a of i)s+=`<option value="${o(a.name)}" ${a.name===r.selectedServer?"selected":""}>
        ${o(a.name)} (${o(a.type)})
      </option>`;s+="</optgroup>"}return s}function z(e){const t=e.usedBytes??(e.uploadBytes??0)+(e.downloadBytes??0),s=e.totalBytes;return s!=null?`${L(t)} / ${L(s)}`:t>0?L(t):"unknown"}function J(e){return r.subscriptions.length?r.subscriptions.map(t=>`
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
        <span>Used <strong>${o(z(t))}</strong></span>
        <span>Down <strong>${o(L(t.downloadBytes))}</strong></span>
        <span>Up <strong>${o(L(t.uploadBytes))}</strong></span>
        <span>Expires <strong>${o(_(t.expireAt))}</strong></span>
        <span>Auto <strong>${o(t.autoUpdateIntervalMinutes)} min</strong></span>
        ${t.profileUpdateIntervalMinutes?`<span>Provider <strong>${o(t.profileUpdateIntervalMinutes)} min</strong></span>`:""}
      </div>
      <div class="sub-actions">
        <label class="toggle">
          <input type="checkbox" data-sub-toggle="${t.id}" ${t.enabled?"checked":""} ${e?"disabled":""}>
          <span></span>
        </label>
        <button data-sub-refresh="${t.id}" ${e||M===t.id?"disabled":""}>
          ${M===t.id?"Refreshing…":"Refresh"}
        </button>
        <button class="danger-button" data-sub-delete="${t.id}" ${e?"disabled":""}>Delete</button>
      </div>
    </article>
  `).join(""):'<div class="sub-empty muted">No subscriptions</div>'}function Q(e){const t=g[e];if(!t)return{text:"Test",cls:""};if(t.loading)return{text:"…",cls:"ping-loading"};if(t.error||t.latencyMs<0)return{text:"Fail",cls:"ping-err",title:t.error};const s=t.latencyMs<600?"ping-ok":t.latencyMs<1800?"ping-mid":"ping-slow";return{text:`${t.latencyMs}ms`,cls:s}}async function W(e){const t=m();if(t){g[e]={loading:!0},p();try{const s=await t.SpeedTestServer(e);g[e]={loading:!1,latencyMs:s.latencyMs,error:s.error||""}}catch(s){g[e]={loading:!1,latencyMs:-1,error:String(s)}}p()}}async function Y(){if(k){q=!0;return}const e=m();if(!e)return;k=!0,q=!1;const t=[...r.servers];w.current=0,w.total=t.length,p();for(const s of t){if(q)break;w.current+=1,g[s.id]={loading:!0},p();try{const n=await e.SpeedTestServer(s.id);g[s.id]={loading:!1,latencyMs:n.latencyMs,error:n.error||""}}catch(n){g[s.id]={loading:!1,latencyMs:-1,error:String(n)}}}k=!1,q=!1,w.current=0,w.total=0,p()}function Z(){const e=r.running?V(r.connectedAt):"";return`
    <header class="topbar">
      <div class="topbar-brand" style="--wails-draggable:drag">
        <h1>MGB VPN</h1>
        <p>Secure tunnel control</p>
      </div>
      <nav class="topbar-nav" style="--wails-draggable:no-drag">
        <button class="nav-tab ${T==="home"?"active":""}" data-nav="home">Home</button>
        <button class="nav-tab ${T==="servers"?"active":""}" data-nav="servers">Servers</button>
      </nav>
      <span class="status ${K()}" style="--wails-draggable:no-drag">
        ${o(r.status)}
        ${e?`<span class="status-elapsed">${o(e)}</span>`:""}
      </span>
    </header>
  `}function ee(){const e=r.running||r.connecting,t=e?"Disconnect":"Connect",s=e?!r.canDisconnect:!r.canConnect,n=F(),i=r.logs.length?r.logs.map(a=>`<div>${o(a)}</div>`).join(""):'<div class="muted">No events yet</div>';return`
    <section class="panel control-panel">
      <label class="field server-field">
        <span>Server</span>
        <div class="server-select-row">
          <select id="server" ${e||!r.servers.length?"disabled":""}>
            ${X()}
          </select>
          <button id="best-server" title="Select fastest tested server" ${e||!n?"disabled":""}>Best</button>
        </div>
      </label>
      <div class="field">
        <span>Mode</span>
        <div class="segmented" role="group" aria-label="Connection mode">
          <button id="mode-proxy" class="${r.mode==="proxy"?"active":""}" ${e?"disabled":""}>Proxy</button>
          <button id="mode-tun" class="${r.mode==="tun"?"active":""}" ${e?"disabled":""}>TUN</button>
        </div>
      </div>
      <button id="primary" class="primary ${r.running?"danger":""}" ${s?"disabled":""}>
        ${t}
      </button>
    </section>

    <section class="info-grid">
      <button class="metric copy-metric" data-copy-proxy="socks" data-copy-value="${o(r.proxySocks)}" title="Copy SOCKS5 address">
        <span>SOCKS5 ${N==="socks"?"<em>Copied</em>":""}</span>
        <strong>${o(r.proxySocks)}</strong>
      </button>
      <button class="metric copy-metric" data-copy-proxy="http" data-copy-value="${o(r.proxyHTTP)}" title="Copy HTTP address">
        <span>HTTP ${N==="http"?"<em>Copied</em>":""}</span>
        <strong>${o(r.proxyHTTP)}</strong>
      </button>
      ${r.running?`
        <div class="metric">
          <span>Session</span>
          <strong>↓ ${o(L(r.traffic.sessionDownloadBytes))} / ↑ ${o(L(r.traffic.sessionUploadBytes))}</strong>
        </div>
        <div class="metric">
          <span>Speed</span>
          <strong>↓ ${o(U(r.traffic.downloadBytesPerSec))} ↑ ${o(U(r.traffic.uploadBytesPerSec))}</strong>
        </div>
      `:""}
    </section>

    <section class="panel subscription-panel">
      <div class="subscription-header">
        <h2>Subscriptions</h2>
        <button id="reload-subscriptions" ${e?"disabled":""}>Reload</button>
      </div>
      <div class="subscription-form">
        <input id="subscription-name" class="sub-f-name" placeholder="Name" value="${o(u.name)}" ${u.busy?"disabled":""}>
        <input id="subscription-url" class="sub-f-url" placeholder="Subscription URL" value="${o(u.url)}" ${u.busy?"disabled":""}>
        <input id="subscription-interval" class="sub-f-interval" type="number" min="1" step="1" value="${o(u.interval)}" ${u.busy?"disabled":""}>
        <label class="check">
          <input id="subscription-enabled" type="checkbox" ${u.enabled?"checked":""} ${u.busy?"disabled":""}>
          <span>Auto</span>
        </label>
        <button id="add-subscription" class="primary sub-f-add" ${u.busy?"disabled":""}>
          ${u.busy?"Adding…":"Add"}
        </button>
      </div>
      <div class="subscription-list">${J(e)}</div>
    </section>

    ${r.status==="TUN is not ready"?`
      <section class="notice">TUN mode requires wintun.dll next to mgb-gui.exe. Rebuild with scripts\\build-gui.bat or scripts\\build-gui.ps1.</section>
    `:""}

    <section class="panel log-panel">
      <div class="log-header">
        <h2>Event Log</h2>
        <button id="reload" ${e?"disabled":""}>Reload servers</button>
      </div>
      <div id="log" class="log">${i}</div>
    </section>
  `}function I(e,t){const s=e.name===r.selectedServer,{text:n,cls:i,title:a}=Q(e.id),l=g[e.id];return`
    <div class="server-card ${s?"selected":""}">
      <span class="type-badge" data-type="${o(e.type.toLowerCase())}">${o(e.type.toUpperCase())}</span>
      <span class="server-name" title="${o(e.name)}">${o(e.name)}</span>
      <span class="server-address">${o(e.address)}</span>
      <div class="server-actions">
        <button class="action-btn use-btn ${s?"use-selected":""}" data-use-server="${o(e.name)}">
          ${s?"✓ Active":"Use"}
        </button>
        <button class="action-btn test-btn ${i}" data-test-server="${e.id}"
          ${l!=null&&l.loading||k?"disabled":""}
          ${a?`title="${o(a)}"`:""}>
          ${n}
        </button>
        ${t?`<button class="action-btn del-btn" data-delete-server="${e.id}" title="Delete server">✕</button>`:""}
      </div>
    </div>
  `}function te(){const{local:e,bySubId:t}=H(),s=B(e),n=r.servers.filter(G).length,i=Object.keys(g).length>0,a=k?`Stop (${w.current}/${w.total})`:"Test All";let l=`
    <div class="page-header">
      <h2>Servers <span class="count-pill">${n}</span></h2>
      <div class="page-header-actions">
        <button id="test-all-btn" class="${k?"test-all-running":""}">${a}</button>
        <button id="sort-speed-btn" class="${C?"active-action":""}" ${i?"":"disabled"}>Sort by speed</button>
        <button id="reload-servers-page">Reload</button>
      </div>
    </div>

    <div class="server-tools">
      <div class="server-import">
        <input id="server-import-source" placeholder="vless://... / ss://... / hysteria2://..." value="${o(v.source)}" ${v.busy?"disabled":""}>
        <button id="server-import-btn" class="primary" ${v.busy||!v.source.trim()?"disabled":""}>
          ${v.busy?"Importing…":"Import"}
        </button>
      </div>
      <input id="server-filter" placeholder="Search servers" value="${o(P)}">
      ${v.message?`<div class="import-message">${o(v.message)}</div>`:""}
      ${v.error?`<div class="import-message error">${o(v.error)}</div>`:""}
    </div>
  `;l+=`
    <div class="server-group">
      <div class="server-group-header">
        <span>Local</span>
        <span class="group-count">${s.length}</span>
      </div>
      ${s.length===0?'<div class="group-empty muted">No local servers.</div>':s.map(d=>I(d,!0)).join("")}
    </div>
  `;for(const d of r.subscriptions){const f=B(t[d.id]||[]);l+=`
      <div class="server-group">
        <div class="server-group-header">
          <span>${o(d.name)}</span>
          <span class="group-count">${f.length}</span>
          <span class="sub-status-badge ${d.enabled?"enabled":"paused"}">${d.enabled?"enabled":"paused"}</span>
        </div>
        ${f.length===0?'<div class="group-empty muted">No servers yet — refresh subscription.</div>':f.map($=>I($,!1)).join("")}
      </div>
    `}for(const[d,f]of Object.entries(t)){if(r.subscriptions.find(x=>x.id==d))continue;const $=B(f);l+=`
      <div class="server-group">
        <div class="server-group-header">
          <span>Subscription #${o(d)}</span>
          <span class="group-count">${$.length}</span>
        </div>
        ${$.length===0?'<div class="group-empty muted">No matching servers.</div>':$.map(x=>I(x,!1)).join("")}
      </div>
    `}return l}function p(){if(j.innerHTML=`
    <div class="shell">
      ${Z()}
      <div class="page-content">
        ${T==="home"?ee():te()}
      </div>
    </div>
  `,se(),T==="home"){const e=document.querySelector("#log");e&&(e.scrollTop=e.scrollHeight)}}function se(){document.querySelectorAll("[data-nav]").forEach(e=>{e.addEventListener("click",()=>{T=e.dataset.nav,p()})}),T==="home"?re():ne()}function re(){var e,t,s,n,i,a,l,d,f,$,x,D;(e=document.querySelector("#server"))==null||e.addEventListener("change",c=>{r.selectedServer=c.target.value,p()}),(t=document.querySelector("#best-server"))==null||t.addEventListener("click",()=>{const c=F();c&&(r.selectedServer=c.name,p())}),(s=document.querySelector("#mode-proxy"))==null||s.addEventListener("click",()=>{r.mode="proxy",p()}),(n=document.querySelector("#mode-tun"))==null||n.addEventListener("click",()=>{r.mode="tun",p()}),(i=document.querySelector("#primary"))==null||i.addEventListener("click",async()=>{const c=m();if(!c)return;const b=r.running||r.connecting?await c.Disconnect():await c.Connect(r.selectedServer,r.mode);y(b)}),(a=document.querySelector("#reload"))==null||a.addEventListener("click",async()=>{const c=m();c&&y(await c.ReloadServers())}),(l=document.querySelector("#reload-subscriptions"))==null||l.addEventListener("click",async()=>{await E()}),document.querySelectorAll("[data-copy-proxy]").forEach(c=>{c.addEventListener("click",async()=>{var S,h;const b=c.dataset.copyValue;if(b)try{(S=window.runtime)!=null&&S.ClipboardSetText?await window.runtime.ClipboardSetText(b):(h=navigator.clipboard)!=null&&h.writeText&&await navigator.clipboard.writeText(b),N=c.dataset.copyProxy,window.clearTimeout(R),R=window.setTimeout(()=>{N="",p()},1200),p()}catch(A){console.error(A)}})}),(d=document.querySelector("#subscription-name"))==null||d.addEventListener("input",c=>{u.name=c.target.value}),(f=document.querySelector("#subscription-url"))==null||f.addEventListener("input",c=>{u.url=c.target.value}),($=document.querySelector("#subscription-interval"))==null||$.addEventListener("input",c=>{u.interval=c.target.value}),(x=document.querySelector("#subscription-enabled"))==null||x.addEventListener("change",c=>{u.enabled=c.target.checked}),(D=document.querySelector("#add-subscription"))==null||D.addEventListener("click",async()=>{const c=m();if(!(!c||!u.url.trim())){u.busy=!0,p();try{const b=Number.parseInt(u.interval,10);await c.AddSubscription({name:u.name.trim()||u.url.trim(),url:u.url.trim(),enabled:u.enabled,autoUpdateIntervalMinutes:Number.isFinite(b)&&b>0?b:1440}),u.name="",u.url="",u.interval="1440",u.enabled=!0,await E(),y(await c.GetState())}finally{u.busy=!1,p()}}}),document.querySelectorAll("[data-sub-refresh]").forEach(c=>{c.addEventListener("click",async()=>{const b=m();if(b){M=Number(c.dataset.subRefresh),p();try{await b.RefreshSubscription(M)}catch(S){console.error(S)}finally{M=null,await E(),y(await b.GetState())}}})}),document.querySelectorAll("[data-sub-delete]").forEach(c=>{c.addEventListener("click",async()=>{const b=m();b&&(await b.DeleteSubscription(Number(c.dataset.subDelete)),await E(),y(await b.GetState()))})}),document.querySelectorAll("[data-sub-toggle]").forEach(c=>{c.addEventListener("change",async()=>{const b=m();if(!b)return;const S=Number(c.dataset.subToggle),h=r.subscriptions.find(A=>A.id===S);h&&(await b.UpdateSubscription(S,{name:h.name,url:h.url,enabled:c.checked,autoUpdateIntervalMinutes:h.autoUpdateIntervalMinutes}),await E())})})}function ne(){var e,t,s,n,i,a;(e=document.querySelector("#reload-servers-page"))==null||e.addEventListener("click",async()=>{const l=m();l&&y(await l.ReloadServers())}),(t=document.querySelector("#test-all-btn"))==null||t.addEventListener("click",()=>{Y()}),(s=document.querySelector("#sort-speed-btn"))==null||s.addEventListener("click",()=>{C=!0,p()}),(n=document.querySelector("#server-filter"))==null||n.addEventListener("input",l=>{const d=l.target.selectionStart;P=l.target.value,p();const f=document.querySelector("#server-filter");f==null||f.focus(),f==null||f.setSelectionRange(d,d)}),(i=document.querySelector("#server-import-source"))==null||i.addEventListener("input",l=>{v.source=l.target.value,v.message="",v.error="";const d=document.querySelector("#server-import-btn");d&&(d.disabled=!v.source.trim())}),(a=document.querySelector("#server-import-btn"))==null||a.addEventListener("click",async()=>{const l=m();if(!(!l||!v.source.trim())){v.busy=!0,v.message="",v.error="",p();try{const d=await l.ImportServers(v.source.trim());v.source="",v.message=d!=null&&d.imported?`Imported ${d.count} server(s).`:"No servers imported.",await E(),y(await l.GetState())}catch(d){v.error=String(d)}finally{v.busy=!1,p()}}}),document.querySelectorAll("[data-use-server]").forEach(l=>{l.addEventListener("click",()=>{r.selectedServer=l.dataset.useServer,T="home",p()})}),document.querySelectorAll("[data-test-server]").forEach(l=>{l.addEventListener("click",()=>{W(Number(l.dataset.testServer))})}),document.querySelectorAll("[data-delete-server]").forEach(l=>{l.addEventListener("click",async()=>{const d=m();if(!d)return;const f=Number(l.dataset.deleteServer);delete g[f],await d.DeleteServer(f),y(await d.ReloadServers())})})}async function ae(){const e=m();if(!e)return;const t=await e.GetServers();t!=null&&t.length&&(r.servers=t,r.selectedServer||(r.selectedServer=t[0].name),p())}async function E(){const e=m();e&&(r.subscriptions=await e.ListSubscriptions()||[],p())}async function oe(){var t,s,n,i;p(),window.setInterval(()=>{r.running&&p()},6e4),(s=(t=window.runtime)==null?void 0:t.EventsOn)==null||s.call(t,"state",a=>y(a)),(i=(n=window.runtime)==null?void 0:n.EventsOn)==null||i.call(n,"log",a=>{a&&!r.logs.includes(a)&&(r.logs=[...r.logs,a].slice(-300),p())});const e=m();e&&(await ae(),await E(),y(await e.GetState()))}oe();
