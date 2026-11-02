const gunsEl = document.getElementById("guns");
const fragmentsEl = document.getElementById("fragments");
const eventsEl = document.getElementById("events");
const markerLayerEl = document.getElementById("markerLayer");
const summaryEl = document.getElementById("summary");
const connectionEl = document.getElementById("connection");

function fmtNumber(value) {
  if (value === null || value === undefined || Number.isNaN(Number(value))) {
    return "-";
  }
  return Number(value).toFixed(1);
}

function fmtText(value) {
  if (value === null || value === undefined || value === "") {
    return "-";
  }
  return String(value);
}

function htmlText(value) {
  return fmtText(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function cssToken(value) {
  return String(value ?? "").toLowerCase().replaceAll("_", "-").replace(/[^a-z0-9-]/g, "");
}

function secondsUntil(timestamp) {
  if (!timestamp) {
    return "-";
  }
  const remaining = (new Date(timestamp).getTime() - Date.now()) / 1000;
  return `${Math.max(0, remaining).toFixed(0)}s`;
}

function localTime(timestamp) {
  if (!timestamp) {
    return "-";
  }
  return new Date(timestamp).toLocaleTimeString();
}

function renderGuns(guns) {
  gunsEl.innerHTML = "";
  for (const gun of guns || []) {
    const card = document.createElement("article");
    card.className = "gun-card";
    card.innerHTML = `
      <div class="gun-title">
        <strong>Gun ${htmlText(gun.id)}</strong>
        <span class="pill state-${cssToken(gun.state)}">${htmlText(gun.state)}</span>
      </div>
      <div class="kv">
        <span>Mode</span><span>${htmlText(gun.mode)}</span>
        <span>Current az/el</span><span>${fmtNumber(gun.current_azimuth)} / ${fmtNumber(gun.current_elevation)}</span>
        <span>Target az/el</span><span>${fmtNumber(gun.target_azimuth)} / ${fmtNumber(gun.target_elevation)}</span>
        <span>Fragment</span><span>${htmlText(gun.assigned_fragment_id)}</span>
        <span>Safety permissive</span><span class="safe-${cssToken(gun.safety_permissive)}">${htmlText(gun.safety_permissive)}</span>
        <span>Fault reason</span><span>${htmlText(gun.safety_fault_reason)}</span>
        <span>Reloading</span><span>${secondsUntil(gun.post_fire_lockout_until)}</span>
      </div>
    `;
    gunsEl.appendChild(card);
  }
}

function renderFragments(fragments) {
  fragmentsEl.innerHTML = "";
  for (const fragment of fragments || []) {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${htmlText(fragment.id)}</td>
      <td class="priority-${cssToken(fragment.priority)}">${htmlText(fragment.priority)}</td>
      <td>${fmtNumber(fragment.azimuth)}</td>
      <td>${fmtNumber(fragment.elevation)}</td>
      <td>${htmlText(fragment.terrain_type)}</td>
      <td>${htmlText(fragment.strategic_zone)}</td>
      <td>${secondsUntil(fragment.impact_at)}</td>
      <td class="status-${cssToken(fragment.status)}">${htmlText(fragment.status)}</td>
      <td>${htmlText(fragment.assigned_gun_id)}</td>
    `;
    fragmentsEl.appendChild(row);
  }
}

function renderMap(fragments) {
  markerLayerEl.innerHTML = "";
  const now = Date.now();
  for (const fragment of fragments || []) {
    if (fragment.status === "INTERCEPTED" && fragment.intercepted_at) {
      const interceptedAgeMs = now - new Date(fragment.intercepted_at).getTime();
      if (interceptedAgeMs > 10000) {
        continue;
      }
    }
    const marker = document.createElement("div");
    marker.className = `marker status-${cssToken(fragment.status)} priority-${cssToken(fragment.priority)}`;
    marker.style.left = `${Math.max(0, Math.min(100, Number(fragment.map_x)))}%`;
    marker.style.top = `${Math.max(0, Math.min(100, Number(fragment.map_y)))}%`;
    marker.title = `${fragment.id} ${fragment.priority} ${fragment.status} ${fmtText(fragment.terrain_type)} ${fmtText(fragment.strategic_zone)}`;
    marker.innerHTML = `<span class="marker-core"></span><span class="marker-label">${htmlText(fragment.id)}</span>`;
    markerLayerEl.appendChild(marker);
  }
}

function renderEvents(events) {
  eventsEl.innerHTML = "";
  for (const event of events || []) {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${htmlText(localTime(event.timestamp))}</td>
      <td class="severity-${cssToken(event.severity)}">${htmlText(event.severity)}</td>
      <td>${htmlText(event.event_type)}</td>
      <td>${htmlText(event.gun_id)}</td>
      <td>${htmlText(event.fragment_id)}</td>
      <td>${htmlText(event.message)}</td>
    `;
    eventsEl.appendChild(row);
  }
}

async function refresh() {
  try {
    const response = await fetch("/api/v1/status", { cache: "no-store" });
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
    const status = await response.json();
    const activeFragments = status.active_fragment_list || [];
    const mapFragments = status.fragment_list || activeFragments;
    renderGuns(status.guns || []);
    renderFragments(activeFragments);
    renderMap(mapFragments);
    renderEvents(status.recent_events || []);
    summaryEl.textContent = `running=${status.running} guns=${status.gun_count} active_fragments=${status.active_fragments} updated=${localTime(status.generated_at)}`;
    connectionEl.textContent = "CONNECTED";
    connectionEl.className = "connection ok";
  } catch (error) {
    summaryEl.textContent = `Status request failed: ${error.message}`;
    connectionEl.textContent = "DISCONNECTED";
    connectionEl.className = "connection bad";
  }
}

refresh();
setInterval(refresh, 1000);
