const $ = (sel) => document.querySelector(sel);

async function api(path, opts = {}) {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
    ...opts,
  });
  const text = await res.text();
  let body;
  try { body = JSON.parse(text); } catch { body = text; }
  if (!res.ok) throw Object.assign(new Error(body.message || res.statusText), { body, status: res.status });
  return body;
}

function setStep(n) {
  document.querySelectorAll("#flowSteps li").forEach((li) => {
    const s = Number(li.dataset.step);
    li.classList.toggle("active", s === n);
    li.classList.toggle("done", s < n);
  });
}

function badge(status) {
  const s = (status || "").toLowerCase();
  return `<span class="badge ${s}">${status || "?"}</span>`;
}

async function refreshOverview() {
  try {
    const o = await api("/api/v1/overview");
    const el = $("#simBanner");
    el.textContent = o.simulated
      ? `BETA ${o.version || ""} · SIMULATED · provider=${o.provider} · ready=${o.readyCount}/${o.snapshotCount}`
      : `BETA ${o.version || ""} · LIVE PATH · provider=${o.provider}`;
    el.classList.toggle("warn", !!o.simulated);
  } catch (e) {
    $("#simBanner").textContent = "API unreachable — start make run-api";
    $("#simBanner").classList.add("warn");
  }
}

async function refreshCatalog() {
  const data = await api("/api/v1/snapshots");
  const root = $("#catalog");
  if (!data.items || !data.items.length) {
    root.textContent = "No snapshots yet.";
    return;
  }
  root.innerHTML = data.items.map((s) => `
    <div class="snap-row">
      <div>
        <div><strong>${s.name}</strong> ${badge(s.status)} ${s.simulated ? '<span class="badge">fake</span>' : ''}</div>
        <div>${s.id}</div>
        <div>${s.hardware?.gpuProduct || ""} · ${s.namespace}</div>
        <div class="snap-actions">
          <button type="button" data-use="${s.id}">Use</button>
          <button type="button" data-hold="${s.id}">Hold</button>
          <button type="button" data-del="${s.id}">Delete</button>
        </div>
      </div>
    </div>`).join("");
  root.querySelectorAll("[data-use]").forEach((btn) => {
    btn.addEventListener("click", () => {
      $("#compatSnap").value = btn.dataset.use;
      $("#restoreSnap").value = btn.dataset.use;
    });
  });
  root.querySelectorAll("[data-hold]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      await api(`/api/v1/snapshots/${btn.dataset.hold}/hold`, { method: "POST", body: JSON.stringify({ hold: true }) });
      await refreshCatalog();
    });
  });
  root.querySelectorAll("[data-del]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      try {
        await api(`/api/v1/snapshots/${btn.dataset.del}`, { method: "DELETE" });
        await refreshAll();
      } catch (e) {
        alert(e.body?.message || e.message);
      }
    });
  });
}

async function refreshOps() {
  const data = await api("/api/v1/operations?limit=15");
  const root = $("#ops");
  if (!data.items?.length) {
    root.textContent = "No operations yet.";
    return;
  }
  root.innerHTML = data.items.map((o) =>
    `${badge(o.status)} ${o.type} ${o.id} → snap ${o.snapshotId}${o.simulated ? " · simulated" : ""}`
  ).join("\n");
}

async function pollOperation(opId, outEl) {
  setStep(2);
  for (let i = 0; i < 40; i++) {
    const op = await api(`/api/v1/operations/${opId}`);
    outEl.textContent = JSON.stringify(op, null, 2);
    if (op.status === "SUCCEEDED" || op.status === "FAILED") return op;
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error("operation poll timeout");
}

async function refreshAll() {
  await refreshOverview();
  await refreshCatalog();
  await refreshOps();
}

$("#captureForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  setStep(1);
  const fd = new FormData(e.target);
  const body = {
    name: fd.get("name"),
    clusterId: fd.get("clusterId"),
    namespace: fd.get("namespace"),
    provider: "fake",
    source: { podName: fd.get("podName"), containerName: fd.get("containerName") },
  };
  try {
    const accepted = await api("/api/v1/snapshots", {
      method: "POST",
      headers: { "Idempotency-Key": `ui-capture-${Date.now()}` },
      body: JSON.stringify(body),
    });
    $("#captureOut").textContent = JSON.stringify(accepted, null, 2);
    $("#compatSnap").value = accepted.snapshotId;
    $("#restoreSnap").value = accepted.snapshotId;
    await pollOperation(accepted.operationId, $("#captureOut"));
    setStep(3);
    await refreshAll();
  } catch (err) {
    $("#captureOut").textContent = JSON.stringify(err.body || { error: err.message }, null, 2);
  }
});

$("#compatForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  setStep(3);
  const fd = new FormData(e.target);
  const id = fd.get("snapshotId");
  try {
    const res = await api(`/api/v1/snapshots/${id}/compatibility`, {
      method: "POST",
      body: JSON.stringify({
        targetClusterId: fd.get("targetClusterId"),
        gpuProduct: fd.get("gpuProduct"),
        driverVersion: fd.get("driverVersion"),
        architecture: fd.get("architecture"),
        imageDigest: fd.get("imageDigest"),
      }),
    });
    $("#compatOut").textContent = JSON.stringify(res, null, 2);
  } catch (err) {
    $("#compatOut").textContent = JSON.stringify(err.body || { error: err.message }, null, 2);
  }
});

$("#restoreForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  setStep(4);
  const fd = new FormData(e.target);
  const id = fd.get("snapshotId");
  try {
    const accepted = await api(`/api/v1/snapshots/${id}/restore`, {
      method: "POST",
      headers: { "Idempotency-Key": `ui-restore-${Date.now()}` },
      body: JSON.stringify({
        targetClusterId: fd.get("targetClusterId"),
        namespace: fd.get("namespace"),
        podName: fd.get("podName"),
        targetNodeName: fd.get("targetNodeName"),
      }),
    });
    $("#restoreOut").textContent = JSON.stringify(accepted, null, 2);
    await pollOperation(accepted.operationId, $("#restoreOut"));
    await refreshAll();
  } catch (err) {
    $("#restoreOut").textContent = JSON.stringify(err.body || { error: err.message }, null, 2);
  }
});

$("#refreshBtn").addEventListener("click", () => refreshAll());
$("#overviewBtn").addEventListener("click", async () => {
  const o = await api("/api/v1/overview");
  $("#catalog").textContent = JSON.stringify(o, null, 2);
});

refreshAll();
setInterval(refreshAll, 5000);
