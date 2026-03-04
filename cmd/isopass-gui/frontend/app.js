const state = {
  serverUrl: "",
  token: "",
  tlsSkipVerify: false,
};

function $(id) {
  return document.getElementById(id);
}

function setStatus(msg) {
  $("status").textContent = msg || "";
}

function clearResults() {
  $("results").innerHTML = "";
}

const MULTI_FIELD_TYPES = {
  username_password: {
    parse: (value) => {
      try {
        const obj = JSON.parse(value);
        return [
          { label: "Username", value: obj.username || "", secret: false },
          { label: "Password", value: obj.password || "", secret: true },
        ];
      } catch (_) { return null; }
    },
  },
  key_value_pair: {
    parse: (value) => {
      try {
        const obj = JSON.parse(value);
        const keys = Object.keys(obj);
        if (keys.length === 0) return null;
        return [
          { label: "Key", value: keys[0], secret: false },
          { label: "Value", value: obj[keys[0]] || "", secret: true },
        ];
      } catch (_) { return null; }
    },
  },
  tls_bundle: {
    parse: (value) => {
      try {
        const obj = JSON.parse(value);
        return [
          { label: "Certificate", value: obj.certificate || "", secret: false },
          { label: "Private Key", value: obj.private_key || "", secret: true },
          { label: "CA Chain", value: obj.ca_chain || "", secret: false },
        ];
      } catch (_) { return null; }
    },
  },
};

const MASK = "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022";

function extractSearchableFields(item) {
  const multiDef = MULTI_FIELD_TYPES[item.type];
  if (!multiDef) return [];
  const parsed = multiDef.parse(item.value || "");
  if (!parsed) return [];
  return parsed.filter((f) => !f.secret).map((f) => f.value.toLowerCase());
}

function renderResults(items) {
  clearResults();
  if (!items || items.length === 0) {
    const empty = document.createElement("div");
    empty.className = "status";
    empty.textContent = "No matches.";
    $("results").appendChild(empty);
    return;
  }
  const tmpl = $("result-template");
  items.forEach((item) => {
    const node = tmpl.content.cloneNode(true);
    const card = node.querySelector(".card");
    const detail = node.querySelector(".card-detail");
    const fieldsContainer = node.querySelector(".value-fields");
    const reveal = node.querySelector(".reveal");
    const copy = node.querySelector(".copy");

    const org = item.organization || item.organization_id || "-";
    const scopes = Array.isArray(item.scopes) && item.scopes.length > 0 ? item.scopes.join(", ") : "all";
    node.querySelector(".pill-org").textContent = org;
    node.querySelector(".pill-scope").textContent = scopes;
    node.querySelector(".pill-type").textContent = item.type || "-";

    const actualValue = item.value || "";
    const multiDef = MULTI_FIELD_TYPES[item.type];
    const parsed = multiDef ? multiDef.parse(actualValue) : null;
    const fields = parsed || [{ label: null, value: actualValue, secret: true }];

    const fieldEls = fields.map((f) => {
      const isSecret = f.secret !== false;
      const wrapper = document.createElement("div");
      wrapper.className = "value-field";
      if (f.label) {
        const lbl = document.createElement("div");
        lbl.className = "value-field-label";
        lbl.textContent = f.label;
        wrapper.appendChild(lbl);
      }
      const val = document.createElement("div");
      val.textContent = isSecret ? MASK : f.value;
      wrapper.appendChild(val);
      fieldsContainer.appendChild(wrapper);
      return { el: val, actual: f.value, secret: isSecret };
    });

    const tagsContainer = node.querySelector(".card-tags");
    const tags = Array.isArray(item.tags) ? item.tags.slice().sort() : [];
    if (tags.length > 0) {
      tags.forEach((tag) => {
        const pill = document.createElement("span");
        pill.className = "pill pill-tag";
        pill.textContent = tag;
        tagsContainer.appendChild(pill);
      });
    }

    function maskAll() {
      fieldEls.forEach((f) => { if (f.secret) f.el.textContent = MASK; });
      reveal.textContent = "Reveal";
    }

    function revealAll() {
      fieldEls.forEach((f) => { if (f.secret) f.el.textContent = f.actual; });
      reveal.textContent = "Hide";
    }

    card.addEventListener("click", (e) => {
      if (e.target.closest(".detail-actions")) return;
      const isOpen = card.classList.toggle("open");
      detail.classList.toggle("hidden", !isOpen);
      if (!isOpen) maskAll();
    });

    reveal.addEventListener("click", () => {
      const isMasked = reveal.textContent === "Reveal";
      if (isMasked) revealAll();
      else maskAll();
    });

    copy.addEventListener("click", () => {
      window.runtime.ClipboardSetText(actualValue);
      setStatus("Copied to clipboard. Clearing in 30s.");
      if (window._clipboardTimer) clearTimeout(window._clipboardTimer);
      window._clipboardTimer = setTimeout(() => {
        window.runtime.ClipboardSetText("");
        setStatus("Clipboard cleared.");
      }, 30000);
    });

    $("results").appendChild(node);
  });
}

async function fetchClientInfo() {
  const el = $("client-info");
  if (!state.serverUrl || !state.token) {
    el.classList.add("hidden");
    return;
  }
  try {
    const data = await window.go.main.App.Connect(
      state.serverUrl, state.token, state.tlsSkipVerify, ""
    );
    el.innerHTML = "";
    const orgs = Array.isArray(data.organizations) ? data.organizations : [];
    const scopes = Array.isArray(data.scopes) ? data.scopes : [];
    if (orgs.length > 0) {
      const row = document.createElement("div");
      row.className = "client-info-row";
      const label = document.createElement("span");
      label.className = "client-info-label";
      label.textContent = "Orgs:";
      row.appendChild(label);
      orgs.forEach((name) => {
        const pill = document.createElement("span");
        pill.className = "pill pill-org";
        pill.textContent = name;
        row.appendChild(pill);
      });
      el.appendChild(row);
    }
    if (scopes.length > 0) {
      const row = document.createElement("div");
      row.className = "client-info-row";
      const label = document.createElement("span");
      label.className = "client-info-label";
      label.textContent = "Scopes:";
      row.appendChild(label);
      scopes.forEach((name) => {
        const pill = document.createElement("span");
        pill.className = "pill pill-scope";
        pill.textContent = name;
        row.appendChild(pill);
      });
      el.appendChild(row);
    }
    el.classList.toggle("hidden", orgs.length === 0 && scopes.length === 0);
  } catch (err) {
    el.innerHTML = "";
    el.textContent = "Connection error: " + err;
    el.classList.remove("hidden");
  }
}

async function saveSettings() {
  state.serverUrl = $("server-url").value.trim();
  state.token = $("token").value.trim();
  state.tlsSkipVerify = $("tls-skip").checked;
  try {
    await window.go.main.App.SaveSettings(
      state.serverUrl, state.token, state.tlsSkipVerify, ""
    );
    setStatus("Settings saved.");
    fetchClientInfo();
    toggleSettings(false);
  } catch (err) {
    setStatus("Save error: " + err);
  }
}

async function runSearch() {
  const tagsRaw = $("search-tags").value.trim();
  const typeFilter = $("search-type").value;
  const searchTerms = tagsRaw
    .split(",")
    .map((t) => t.trim().toLowerCase())
    .filter(Boolean);
  if (!state.serverUrl || !state.token) {
    setStatus("Configure server URL and token first.");
    toggleSettings(true);
    return;
  }
  setStatus("Searching...");
  clearResults();
  try {
    let data = await window.go.main.App.Search(200);
    if (!data) data = [];
    if (typeFilter) {
      data = data.filter((item) => item.type === typeFilter);
    }
    if (searchTerms.length > 0) {
      data = data.filter((item) => {
        const itemTags = (Array.isArray(item.tags) ? item.tags : []).map((t) => t.toLowerCase());
        const extraFields = extractSearchableFields(item);
        const allSearchable = [...itemTags, ...extraFields];
        return searchTerms.every((term) => allSearchable.some((s) => s.includes(term)));
      });
    }
    setStatus(`Found ${data.length} result(s).`);
    renderResults(data);
  } catch (err) {
    setStatus(`Error: ${err}`);
  }
}

function toggleSettings(force) {
  const panel = $("settings");
  if (!panel) return;
  const search = $("search");
  const toggleBtn = $("settings-toggle");
  const show = typeof force === "boolean" ? force : panel.classList.contains("hidden");
  panel.classList.toggle("hidden", !show);
  panel.style.display = show ? "block" : "none";
  if (search) search.classList.toggle("dim", show);
  if (toggleBtn) toggleBtn.textContent = show ? "Close" : "Settings";
  if (show) {
    fetchClientInfo();
    checkOIDCStatus();
  }
}

async function checkOIDCStatus() {
  const ssoBtn = $("sso-login");
  if (!ssoBtn) return;
  const serverUrl = $("server-url") ? $("server-url").value.trim() : state.serverUrl;
  if (!serverUrl) {
    ssoBtn.classList.add("hidden");
    return;
  }
  try {
    const enabled = await window.go.main.App.CheckOIDCStatus(serverUrl);
    ssoBtn.classList.toggle("hidden", !enabled);
  } catch (_) {
    ssoBtn.classList.add("hidden");
  }
}

async function openSSOLogin() {
  const serverUrl = $("server-url") ? $("server-url").value.trim() : state.serverUrl;
  if (!serverUrl) return;
  const url = await window.go.main.App.OIDCAuthorizeURL(serverUrl);
  window.runtime.BrowserOpenURL(url);
}

function bindEvents() {
  const settingsToggle = $("settings-toggle");
  if (settingsToggle) settingsToggle.addEventListener("click", () => toggleSettings());
  const saveBtn = $("save-settings");
  if (saveBtn) saveBtn.addEventListener("click", saveSettings);
  const ssoBtn = $("sso-login");
  if (ssoBtn) ssoBtn.addEventListener("click", openSSOLogin);
  const searchBtn = $("run-search");
  if (searchBtn) searchBtn.addEventListener("click", runSearch);
  const tagsInput = $("search-tags");
  if (tagsInput) {
    tagsInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") runSearch();
    });
  }
  const typeSelect = $("search-type");
  if (typeSelect) {
    typeSelect.addEventListener("change", () => {
      if ($("search-tags").value.trim() || typeSelect.value) runSearch();
    });
  }
  const tokenToggle = $("token-toggle");
  if (tokenToggle) {
    tokenToggle.addEventListener("click", () => {
      const input = $("token");
      if (!input) return;
      const isHidden = input.type === "password";
      input.type = isHidden ? "text" : "password";
      tokenToggle.textContent = isHidden ? "Hide" : "Reveal";
    });
  }
}

async function init() {
  bindEvents();
  try {
    const cfg = await window.go.main.App.LoadSettings();
    if (cfg) {
      state.serverUrl = cfg.server_url || "";
      state.token = cfg.bearer_token || "";
      state.tlsSkipVerify = cfg.tls_skip_verify || false;
      $("server-url").value = state.serverUrl;
      $("token").value = state.token;
      $("tls-skip").checked = state.tlsSkipVerify;
    }
  } catch (_) {
    // No config yet — that's fine.
  }
  if (!state.serverUrl || !state.token) {
    toggleSettings(true);
  } else {
    fetchClientInfo();
  }
}

document.addEventListener("DOMContentLoaded", init);
