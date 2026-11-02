const fs = require("fs");
const path = require("path");
const sqlite3 = require("sqlite3").verbose();

const dbPath = process.env.FUXA_PROJECT_DB || "/usr/src/app/FUXA/server/_appdata/project.fuxap.db";

const colors = {
  body: "#0f172aff",
  header: "#111827ff",
  card: "#1f2937ff",
  panel: "#111827ff",
  border: "#374151ff",
  text: "#e5e7ebff",
  muted: "#9ca3afff",
  online: "#22c55eff",
  warning: "#f59e0bff",
  fault: "#ef4444ff",
  offline: "#6b7280ff",
  accent: "#60a5faff",
  amberPanel: "#451a03ff",
  amberBorder: "#92400eff"
};

const fontStack = "Arial, Helvetica, sans-serif";

const gunIps = {
  1: "172.28.0.10",
  2: "172.28.0.12",
  3: "172.28.0.13",
  4: "172.28.0.14",
  5: "172.28.0.15",
  6: "172.28.0.16",
  7: "172.28.0.17",
  8: "172.28.0.18"
};

const gunLimits = {
  1: { minAzimuth: 0, maxAzimuth: 45, minElevation: 5, maxElevation: 85 },
  2: { minAzimuth: 45, maxAzimuth: 90, minElevation: 5, maxElevation: 85 },
  3: { minAzimuth: 90, maxAzimuth: 135, minElevation: 5, maxElevation: 85 },
  4: { minAzimuth: 135, maxAzimuth: 180, minElevation: 5, maxElevation: 85 },
  5: { minAzimuth: 180, maxAzimuth: 225, minElevation: 5, maxElevation: 85 },
  6: { minAzimuth: 225, maxAzimuth: 270, minElevation: 5, maxElevation: 85 },
  7: { minAzimuth: 270, maxAzimuth: 315, minElevation: 5, maxElevation: 85 },
  8: { minAzimuth: 315, maxAzimuth: 359, minElevation: 5, maxElevation: 85 }
};

const statusTags = [
  ["accepted_sequence_low", "AcceptedSequenceLow", "UInt16", 21, "Accepted command sequence low word"],
  ["accepted_sequence_high", "AcceptedSequenceHigh", "UInt16", 22, "Accepted command sequence high word"],
  ["state", "State", "UInt16", 23, "Operating state"],
  ["current_azimuth", "CurrentAzimuth", "Int16", 24, "Current azimuth x10"],
  ["current_elevation", "CurrentElevation", "Int16", 25, "Current elevation x10"],
  ["active_fragment_low", "ActiveFragmentLow", "UInt16", 26, "Active fragment low word"],
  ["active_fragment_high", "ActiveFragmentHigh", "UInt16", 27, "Active fragment high word"],
  ["target_accepted", "TargetAccepted", "UInt16", 28, "Target accepted pulse"],
  ["on_target", "OnTarget", "UInt16", 29, "On-target status"],
  ["settling", "Settling", "UInt16", 30, "Settling status"],
  ["firing", "Firing", "UInt16", 31, "Firing status"],
  ["lockout", "Reloading", "UInt16", 32, "Post-fire reloading status"],
  ["ready", "Ready", "UInt16", 33, "Ready status"],
  ["fault_active", "FaultActive", "UInt16", 34, "Fault active status"],
  ["fault_code", "FaultCode", "UInt16", 35, "Fault code"],
  ["shot_sequence_low", "ShotSequenceLow", "UInt16", 36, "Shot sequence low word"],
  ["shot_sequence_high", "ShotSequenceHigh", "UInt16", 37, "Shot sequence high word"],
  ["settling_seconds", "SettlingSeconds", "UInt16", 38, "Settling seconds remaining"],
  ["lockout_seconds", "ReloadingSeconds", "UInt16", 39, "Reloading seconds remaining"]
];

function tagId(gun, key) {
  return `t_auto_gun${gun}_${key}`;
}

function valueId(gun, key) {
  return `VAL_auto_gun${gun}_${key}`;
}

function ledId(gun, key) {
  return `GSE_auto_gun${gun}_${key}`;
}

function buttonId(gun) {
  return `HXB_auto_gun${gun}_fault_reset`;
}

function manualModeButtonId(gun) {
  return `HXB_manual_gun${gun}_request_manual`;
}

function autoResetScriptId(gun) {
  return `s_stonehenge_gun${gun}_auto_fault_reset`;
}

function manualModeScriptId(gun) {
  return `s_stonehenge_gun${gun}_request_manual_mode`;
}

function navScriptId(gun, kind) {
  return `s_stonehenge_nav_gun${gun}_${kind}`;
}

function hmiLocalTagId(gun, key) {
  return `t_stonehenge_gun${gun}_hmi_${key}`;
}

function hmiPlaceholderId(gun, key) {
  return `@stonehenge_gun${gun}_hmi_${key}`;
}

function hmiInputId(gun, key) {
  return `HXI_stonehenge_gun${gun}_${key}`;
}

function hmiButtonId(gun, key) {
  return `HXB_stonehenge_gun${gun}_${key}`;
}

function hmiSwitchId(gun, key) {
  return `HXT_stonehenge_gun${gun}_${key}`;
}

function hmiSetButtonId(gun, key, value) {
  return `HXB_stonehenge_gun${gun}_${key}_${value ? "unlock" : "lock"}`;
}

function navButtonId(gun, kind) {
  return `HXB_stonehenge_nav_gun${gun}_${kind}`;
}

function hmiLocalValueId(gun, key) {
  return `VAL_stonehenge_gun${gun}_hmi_${key}`;
}

function hmiSetScriptId(gun, key, value) {
  return `s_stonehenge_gun${gun}_set_${key}_${value ? "on" : "off"}`;
}

function adminManualViewId(gun) {
  return `v_stonehenge_gun${gun}_admin_manual`;
}

function viewName(gun, kind) {
  if (kind === "auto") return `Gun ${gun} Auto HMI`;
  if (kind === "manual") return `Gun ${gun} Manual HMI`;
  if (kind === "admin") return `Gun ${gun} Admin Manual HMI`;
  throw new Error(`unknown view kind: ${kind}`);
}

function sqlRun(db, sql, params = []) {
  return new Promise((resolve, reject) => {
    db.run(sql, params, function onRun(err) {
      if (err) reject(err);
      else resolve(this);
    });
  });
}

function sqlGet(db, sql, params = []) {
  return new Promise((resolve, reject) => {
    db.get(sql, params, (err, row) => {
      if (err) reject(err);
      else resolve(row);
    });
  });
}

function buildGunDevice(gun) {
  const tags = {};
  for (const [key, label, type, address, description] of statusTags) {
    tags[tagId(gun, key)] = {
      id: tagId(gun, key),
      name: `Gun${gun}_${label}`,
      label: "",
      type,
      memaddress: "400000",
      address: String(address),
      divisor: 1,
      options: "",
      format: null,
      daq: {
        enabled: false,
        interval: 60,
        changed: true,
        restored: false
      },
      description: `Gun ${gun} ${description}`
    };
  }

  return {
    id: `d_stonehenge_gun${gun}_plc`,
    name: `Stonehenge Gun ${gun} PLC`,
    enabled: true,
    property: {
      address: `${gunIps[gun]}:502`,
      baudrate: "9600",
      databits: "8",
      delay: 10,
      forceFC16: false,
      parity: "None",
      port: "",
      rack: "",
      slaveid: "1",
      slot: "",
      stopbits: "1"
    },
    type: "ModbusTCP",
    polling: 500,
    tags
  };
}

function buildFuxaServerDevice(existing = {}) {
  const tags = { ...(existing.tags || {}) };
  for (let gun = 1; gun <= 8; gun += 1) {
    const localTags = [
      ["manual_azimuth", "number", 0],
      ["manual_elevation", "number", 50],
      ["admin_azimuth", "number", 0],
      ["admin_elevation", "number", 50],
      ["admin_override", "number", 0],
      ["extended_permissive", "number", 0],
      ["admin_override_state", "string", "LOCKED"],
      ["extended_permissive_state", "string", "LOCKED"],
      ["admin_mode_state", "string", "UNKNOWN"],
      ["admin_action_state", "string", "READY"]
    ];
    for (const [key, type, value] of localTags) {
      const id = hmiLocalTagId(gun, key);
      tags[id] = {
        id,
        name: `Gun ${gun} HMI ${key.replace(/_/g, " ")}`,
        label: `Gun ${gun} HMI ${key.replace(/_/g, " ")}`,
        value,
        type,
        memaddress: "",
        address: "",
        divisor: 1,
        options: "",
        format: null,
        daq: {
          enabled: false,
          interval: 60,
          changed: false,
          restored: false
        },
        init: String(value),
        sysType: 1,
        description: "Stonehenge FUXA-local HMI field"
      };
    }
  }
  return {
    id: "0",
    name: "FUXA Server",
    enabled: true,
    property: existing.property || {},
    type: "FuxaServer",
    polling: existing.polling || 1000,
    tags
  };
}

function itemValue(gun, key, name) {
  return {
    id: valueId(gun, key),
    type: "svg-ext-value",
    name,
    property: {
      variableId: tagId(gun, key),
      ranges: [
        {
          min: -32768,
          max: 65535,
          textId: null,
          color: "",
          type: 1,
          stroke: "",
          fractionDigitsId: null,
          fractionDigits: "0"
        }
      ],
      events: [],
      actions: []
    },
    label: "Value",
    hide: false,
    lock: false
  };
}

function itemHmiLocalValue(gun, key, name) {
  return {
    id: hmiLocalValueId(gun, key),
    type: "svg-ext-value",
    name,
    property: {
      variableId: hmiLocalTagId(gun, key),
      ranges: [
        {
          min: -32768,
          max: 65535,
          textId: null,
          color: "",
          type: 1,
          stroke: "",
          fractionDigitsId: null,
          fractionDigits: "0"
        }
      ],
      events: [],
      actions: []
    },
    label: "Value",
    hide: false,
    lock: false
  };
}

function itemLed(gun, key, name, activeColor) {
  return {
    id: ledId(gun, key),
    type: "svg-ext-gauge_semaphore",
    name,
    property: {
      variableId: tagId(gun, key),
      ranges: [
        { min: 0, max: 0, color: colors.offline, type: 2, stroke: "#4b5563ff" },
        { min: 1, max: 1, color: activeColor, type: 2, stroke: activeColor }
      ],
      events: [],
      actions: []
    },
    label: "HtmlSemaphore",
    hide: false,
    lock: false
  };
}

function itemButton(gun) {
  return {
    id: buttonId(gun),
    type: "svg-ext-html_button",
    name: `Gun${gun}_Auto_Fault_Reset`,
    property: {
      ranges: [{ min: 20, max: 80, color: "", type: 2, stroke: "" }],
      events: [
        {
          type: "click",
          action: "onRunScript",
          actparam: autoResetScriptId(gun),
          actoptions: { params: [] }
        }
      ],
      actions: [],
      text: "FAULT RESET"
    },
    label: "HtmlButton",
    hide: false,
    lock: false
  };
}

function itemManualModeButton(gun) {
  return {
    id: manualModeButtonId(gun),
    type: "svg-ext-html_button",
    name: `Gun${gun}_Request_Manual_Mode`,
    property: {
      ranges: [{ min: 20, max: 80, color: "", type: 2, stroke: "" }],
      events: [
        {
          type: "click",
          action: "onRunScript",
          actparam: manualModeScriptId(gun),
          actoptions: { params: [] }
        }
      ],
      actions: [],
      text: "REQUEST MANUAL MODE"
    },
    label: "HtmlButton",
    hide: false,
    lock: false
  };
}

function itemHmiInput(gun, key, name, min, max) {
  return {
    id: hmiInputId(gun, key),
    type: "svg-ext-html_input",
    name,
    property: {
      variableId: hmiLocalTagId(gun, key),
      events: [],
      actions: [],
      options: {
        updated: true,
        numeric: true,
        type: "number",
        maxlength: null,
        readonly: false,
        max,
        min,
        selectOnClick: true,
        actionOnEsc: "update"
      }
    },
    label: "HtmlInput",
    hide: false,
    lock: false
  };
}

function itemHmiSwitch(gun, key, name) {
  return {
    id: hmiSwitchId(gun, key),
    type: "svg-ext-html_switch",
    name,
    property: {
      variableId: hmiLocalTagId(gun, key),
      events: [],
      actions: [],
      options: {
        offValue: 0,
        onValue: 1,
        offBackground: "#334155",
        onBackground: "#7f1d1d",
        offText: "LOCKED",
        onText: "UNLOCKED",
        offSliderColor: "#94a3b8",
        onSliderColor: "#ef4444",
        offTextColor: "#e5e7eb",
        onTextColor: "#fee2e2",
        fontSize: 11,
        fontFamily: fontStack,
        radius: 6,
        height: 28
      }
    },
    label: "HtmlSwitch",
    hide: false,
    lock: false
  };
}

function itemHmiSetButton(gun, key, value, label) {
  return {
    id: hmiSetButtonId(gun, key, value),
    type: "svg-ext-html_button",
    name: label,
    property: {
      ranges: [
        { min: 0, max: 0, color: "", type: 2, stroke: "" },
        { min: 1, max: 1, color: "", type: 2, stroke: "" }
      ],
      events: [
        {
          type: "click",
          action: "onRunScript",
          actparam: hmiSetScriptId(gun, key, value),
          actoptions: { params: [] }
        }
      ],
      actions: [],
      text: label
    },
    label: "HtmlButton",
    hide: false,
    lock: false
  };
}

function itemHmiButton(gun, key, name) {
  return {
    id: hmiButtonId(gun, key),
    type: "svg-ext-html_button",
    name,
    property: {
      ranges: [{ min: 20, max: 80, color: "", type: 2, stroke: "" }],
      events: [],
      actions: [],
      text: name
    },
    label: "HtmlButton",
    hide: false,
    lock: false
  };
}

function itemNavButton(gun, kind, label) {
  return {
    id: navButtonId(gun, kind),
    type: "svg-ext-html_button",
    name: label,
    property: {
      ranges: [{ min: 20, max: 80, color: "", type: 2, stroke: "" }],
      events: [
        {
          type: "click",
          action: "onRunScript",
          actparam: navScriptId(gun, kind),
          actoptions: { params: [] }
        }
      ],
      actions: [],
      text: label
    },
    label: "HtmlButton",
    hide: false,
    lock: false
  };
}

function text(id, x, y, content, size = 18, fill = colors.text, weight = "500", anchor = "start") {
  const numericWeight = Number.parseInt(weight, 10);
  const cleanWeight = Number.isFinite(numericWeight) && numericWeight > 700 ? "700" : weight;
  return `<text id="${id}" x="${x}" y="${y}" fill="${fill}" font-size="${size}" font-family="${fontStack}" font-weight="${cleanWeight}" text-anchor="${anchor}">${content}</text>`;
}

function rect(id, x, y, width, height, fill = colors.card, stroke = colors.border, rx = 8, sw = 1.5) {
  return `<rect id="${id}" x="${x}" y="${y}" width="${width}" height="${height}" rx="${rx}" fill="${fill}" stroke="${stroke}" stroke-width="${sw}"/>`;
}

function attr(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function hmiInputSelector(gun, key) {
  return `#${hmiInputId(gun, key)} input`;
}

function hmiClientClick(gun, key) {
  const limits = gunLimits[gun];
  const setLocalText = (elementId, value, fill = "#e5e7eb") => `const ${elementId.replace(/[^A-Za-z0-9_$]/g, "_")}=document.querySelector('#${elementId}_text');if(${elementId.replace(/[^A-Za-z0-9_$]/g, "_")}){${elementId.replace(/[^A-Za-z0-9_$]/g, "_")}.textContent=${JSON.stringify(value)};${elementId.replace(/[^A-Za-z0-9_$]/g, "_")}.setAttribute('fill','${fill}');}`;
  const actionElement = hmiLocalValueId(gun, "admin_action_state");
  const modeElement = hmiLocalValueId(gun, "admin_mode_state");

  if (key === "manual_stage") {
    return [
      "event.stopPropagation();",
      "event.preventDefault();",
      `const az=Number(document.querySelector('${hmiInputSelector(gun, "manual_azimuth")}')?.value);`,
      `const el=Number(document.querySelector('${hmiInputSelector(gun, "manual_elevation")}')?.value);`,
      `if(!Number.isFinite(az)||!Number.isFinite(el)){${setLocalText(actionElement, "COORDS REQUIRED", "#fecaca")}return false;}`,
      `if(az<${limits.minAzimuth}||az>${limits.maxAzimuth}){${setLocalText(actionElement, "AZIMUTH LIMIT", "#fecaca")}return false;}`,
      `if(el<${limits.minElevation}||el>${limits.maxElevation}){${setLocalText(actionElement, "ELEVATION LIMIT", "#fecaca")}return false;}`,
      setLocalText(actionElement, "STAGING TARGET", "#bfdbfe"),
      `fetch('/stonehenge/hmi/guns/${gun}/manual',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({target_azimuth:az,target_elevation:el,fire_request:false,admin_override:false,safety_extended_permissive:false})}).then(async r=>{const j=await r.json().catch(()=>({}));window.stonehengeLastCommand=j;${setLocalText(actionElement, "TARGET STAGED", "#86efac")}if(!r.ok){${setLocalText(actionElement, "COMMAND DENIED", "#fecaca")}}}).catch(e=>{window.stonehengeLastCommand={error:String(e)};${setLocalText(actionElement, "COMMAND FAILED", "#fecaca")}});`,
      "return false;"
    ].join("");
  }

  const adminManualCommand = (fire) => [
    "event.stopPropagation();",
    "event.preventDefault();",
    `const az=Number(document.querySelector('${hmiInputSelector(gun, "admin_azimuth")}')?.value);`,
    `const el=Number(document.querySelector('${hmiInputSelector(gun, "admin_elevation")}')?.value);`,
    `const state=window.__stonehengeAdminState=window.__stonehengeAdminState||{};`,
    `const admin=state['g${gun}_admin_override']===true;`,
    `const ext=state['g${gun}_extended_permissive']===true;`,
    `const minEl=(admin&&ext)?-30:${limits.minElevation};`,
    `const maxEl=(admin&&ext)?90:${limits.maxElevation};`,
    `if(!Number.isFinite(az)||!Number.isFinite(el)){${setLocalText(actionElement, "COORDS REQUIRED", "#fecaca")}return false;}`,
    `if(az<${limits.minAzimuth}||az>${limits.maxAzimuth}){${setLocalText(actionElement, "AZIMUTH LIMIT", "#fecaca")}return false;}`,
    `if(el<minEl||el>maxEl){${setLocalText(actionElement, "ELEVATION LIMIT", "#fecaca")}return false;}`,
    fire ? `if(!(admin&&ext)){${setLocalText(actionElement, "FIRE BLOCKED", "#fecaca")}return false;}` : "void 0;",
    `fetch('/stonehenge/hmi/guns/${gun}/manual',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({target_azimuth:az,target_elevation:el,fire_request:${fire ? "true" : "false"},admin_override:admin,safety_extended_permissive:ext})}).then(async r=>{const j=await r.json().catch(()=>({}));window.stonehengeLastCommand=j;${setLocalText(actionElement, fire ? "FIRE ACCEPTED" : "TARGET STAGED", "#86efac")}if(!r.ok){${setLocalText(actionElement, "COMMAND DENIED", "#fecaca")}}}).catch(e=>{window.stonehengeLastCommand={error:String(e)};${setLocalText(actionElement, "COMMAND FAILED", "#fecaca")}});`,
    "return false;"
  ].join("");

  if (key === "admin_stage") return adminManualCommand(false);
  if (key === "admin_fire") return adminManualCommand(true);

  const adminModePrefix = "admin_mode_";
  const manualModePrefix = "manual_mode_";
  if (key.startsWith(adminModePrefix) || key.startsWith(manualModePrefix)) {
    const prefix = key.startsWith(adminModePrefix) ? adminModePrefix : manualModePrefix;
    const mode = key.slice(prefix.length) === "maintenance" ? "maintenance" : key.slice(prefix.length);
    const modeLabel = mode.toUpperCase();
    return [
      "event.stopPropagation();",
      "event.preventDefault();",
      setLocalText(modeElement, "REQUESTING", "#bfdbfe"),
      setLocalText(actionElement, `REQUESTING ${modeLabel}`, "#bfdbfe"),
      `fetch('/stonehenge/hmi/guns/${gun}/mode',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({mode:'${mode}'})}).then(async r=>{const j=await r.json().catch(()=>({}));window.stonehengeLastCommand=j;if(r.ok){${setLocalText(modeElement, modeLabel, "#86efac")}${setLocalText(actionElement, `MODE ${modeLabel} OK`, "#86efac")}}else{${setLocalText(actionElement, "MODE DENIED", "#fecaca")}}}).catch(e=>{window.stonehengeLastCommand={error:String(e)};${setLocalText(actionElement, "MODE FAILED", "#fecaca")}});`,
      "return false;"
    ].join("");
  }

  return "";
}

function middlewareScript(id, name, path, method, payload) {
  const bodyLine = payload
    ? `    body: JSON.stringify(${JSON.stringify(payload)})\n`
    : "";
  return {
    id,
    name,
    sync: false,
    parameters: [],
    mode: "SERVER",
    code: [
      `const response = await fetch("http://middleware:8080${path}", {`,
      `    method: "${method}",`,
      "    headers: {",
      "        \"Content-Type\": \"application/json\",",
      "        \"Accept\": \"application/json\",",
      "        \"X-Stonehenge-Control-Token\": process.env.STONEHENGE_CONTROL_TOKEN || \"\"",
      "    },",
      bodyLine.trimEnd(),
      "});",
      "",
      "const text = await response.text();",
      "let body = {};",
      "if (text) {",
      "    try { body = JSON.parse(text); } catch { body = { message: text }; }",
      "}",
      "if (!response.ok) {",
      "    const reason = body.reason || body.error || body.message || text || `HTTP ${response.status}`;",
      "    throw new Error(reason);",
      "}",
      `console.log("${name}:", JSON.stringify(body));`,
      "return body;"
    ].filter(line => line !== "").join("\n")
  };
}

function buildScripts(gun) {
  return [
    middlewareScript(
      autoResetScriptId(gun),
      `resetGun${gun}AutoFault`,
      `/api/v1/guns/${gun}/fault-reset`,
      "POST",
      { resume_auto: true }
    ),
    middlewareScript(
      manualModeScriptId(gun),
      `requestGun${gun}ManualMode`,
      `/api/v1/guns/${gun}/mode`,
      "PUT",
      { mode: "manual" }
    ),
    hmiLocalSetScript(
      hmiSetScriptId(gun, "admin_override", false),
      `setGun${gun}AdminOverrideOff`,
      hmiLocalTagId(gun, "admin_override"),
      0,
      hmiLocalTagId(gun, "admin_override_state"),
      "LOCKED",
      hmiLocalTagId(gun, "admin_action_state")
    ),
    hmiLocalSetScript(
      hmiSetScriptId(gun, "admin_override", true),
      `setGun${gun}AdminOverrideOn`,
      hmiLocalTagId(gun, "admin_override"),
      1,
      hmiLocalTagId(gun, "admin_override_state"),
      "UNLOCKED",
      hmiLocalTagId(gun, "admin_action_state")
    ),
    hmiLocalSetScript(
      hmiSetScriptId(gun, "extended_permissive", false),
      `setGun${gun}ExtendedPermissiveOff`,
      hmiLocalTagId(gun, "extended_permissive"),
      0,
      hmiLocalTagId(gun, "extended_permissive_state"),
      "LOCKED",
      hmiLocalTagId(gun, "admin_action_state")
    ),
    hmiLocalSetScript(
      hmiSetScriptId(gun, "extended_permissive", true),
      `setGun${gun}ExtendedPermissiveOn`,
      hmiLocalTagId(gun, "extended_permissive"),
      1,
      hmiLocalTagId(gun, "extended_permissive_state"),
      "UNLOCKED",
      hmiLocalTagId(gun, "admin_action_state")
    )
  ];
}

function retiredDirectMiddlewareScriptIds(gun) {
  return [
    `s_stonehenge_gun${gun}_stage_manual_target`,
    `s_stonehenge_gun${gun}_stage_admin_manual_target`,
    `s_stonehenge_gun${gun}_fire_admin_manual`,
    `s_stonehenge_gun${gun}_admin_set_manual_mode`,
    `s_stonehenge_gun${gun}_admin_set_auto_mode`
  ];
}

function navScript(gun, kind) {
  return {
    id: navScriptId(gun, kind),
    name: `navGun${gun}${kind.charAt(0).toUpperCase()}${kind.slice(1)}`,
    sync: true,
    parameters: [],
    mode: "CLIENT",
    code: [
      `$setView(${JSON.stringify(viewName(gun, kind))});`,
      "return true;"
    ].join("\n")
  };
}

function hmiLocalSetScript(id, name, tagIdValue, value, stateTagIdValue, stateValue, actionTagIdValue) {
  return {
    id,
    name,
    sync: false,
    parameters: [],
    mode: "SERVER",
    code: [
      `const ok = await $setTag("${tagIdValue}", ${value});`,
      `const stateOk = await $setTag("${stateTagIdValue}", ${JSON.stringify(stateValue)});`,
      `await $setTag("${actionTagIdValue}", ${JSON.stringify(`${stateValue}: ${tagIdValue.replace(/^t_stonehenge_gun\d+_hmi_/, "").replace(/_/g, " ").toUpperCase()}`)});`,
      `console.log("${name}: ${tagIdValue}=${value} ok=" + ok);`,
      `return { tag: "${tagIdValue}", value: ${value}, state: ${JSON.stringify(stateValue)}, ok, stateOk };`
    ].join("\n")
  };
}

function valueSvg(gun, key, x, y, width = 110) {
  return [
    rect(`box_${gun}_${key}`, x, y - 26, width, 34, "#0b1220ff", "#475569ff", 5, 1),
    `<g data-name="Gun${gun}_${key}_Value" id="${valueId(gun, key)}" type="svg-ext-value" fill="${colors.text}" stroke="${colors.text}" font-size="17" stroke-width="0" font-family="${fontStack}" text-anchor="middle">`,
    `<text id="${valueId(gun, key)}_text" x="${x + width / 2}" y="${y - 3}" fill="${colors.text}" stroke="${colors.text}" stroke-width="0" font-size="17" font-family="${fontStack}" text-anchor="middle">--</text>`,
    "</g>"
  ].join("\n");
}

function hmiLocalValueSvg(gun, key, x, y, width = 42) {
  return [
    rect(`box_hmi_${gun}_${key}`, x, y - 22, width, 26, "#0b1220ff", "#475569ff", 5, 1),
    `<g data-name="Gun${gun}_${key}_LocalValue" id="${hmiLocalValueId(gun, key)}" type="svg-ext-value" fill="${colors.text}" stroke="${colors.text}" font-size="12" stroke-width="0" font-family="${fontStack}" text-anchor="middle">`,
    `<text id="${hmiLocalValueId(gun, key)}_text" x="${x + width / 2}" y="${y - 4}" fill="${colors.text}" stroke="${colors.text}" stroke-width="0" font-size="12" font-family="${fontStack}" font-weight="700" text-anchor="middle">--</text>`,
    "</g>"
  ].join("\n");
}

function ledSvg(gun, key, label, x, y) {
  return [
    `<g data-name="Gun${gun}_${key}_Led" id="${ledId(gun, key)}" type="svg-ext-gauge_semaphore" fill="${colors.offline}" stroke="#4b5563ff" stroke-width="2">`,
    `<ellipse id="${ledId(gun, key)}_ellipse" cx="${x}" cy="${y}" rx="15" ry="15" fill="${colors.offline}" stroke="#4b5563ff" stroke-width="2"/>`,
    "</g>",
    text(`lbl_${gun}_${key}`, x + 28, y + 6, label, 16, colors.text, "700")
  ].join("\n");
}

function resetButtonSvg(gun) {
  return [
    `<g data-name="Gun${gun}_Auto_Fault_Reset" id="${buttonId(gun)}" type="svg-ext-html_button" fill="rgba(0,0,0,0)" stroke="rgba(0,0,0,0)" style="cursor:pointer;pointer-events:all;">`,
    `<rect id="${buttonId(gun)}_rect" x="982" y="558" width="226" height="46" rx="8" fill="rgba(0,0,0,0.01)" stroke-width="0" style="pointer-events:all;"/>`,
    `<foreignObject id="${buttonId(gun)}_fo" x="982" y="558" width="226" height="46" style="pointer-events:none;">`,
    `<button xmlns="http://www.w3.org/1999/xhtml" tabindex="-1" style="pointer-events:none;width:calc(100% - 6px);height:calc(100% - 6px);margin:3px;text-align:center;background-color:#451a03;color:#fef3c7;border:1px solid #f59e0b;border-radius:7px;font-size:13px;font-family:${fontStack};font-weight:700;letter-spacing:.02em;">FAULT RESET</button>`,
    "</foreignObject>",
    "</g>"
  ].join("\n");
}

function buildAutoView(gun) {
  const items = {
    [valueId(gun, "state")]: itemValue(gun, "state", `Gun${gun}_State_Display`),
    [valueId(gun, "current_azimuth")]: itemValue(gun, "current_azimuth", `Gun${gun}_CurrentAzimuth_Display`),
    [valueId(gun, "current_elevation")]: itemValue(gun, "current_elevation", `Gun${gun}_CurrentElevation_Display`),
    [valueId(gun, "active_fragment_low")]: itemValue(gun, "active_fragment_low", `Gun${gun}_ActiveFragmentLow_Display`),
    [valueId(gun, "fault_code")]: itemValue(gun, "fault_code", `Gun${gun}_FaultCode_Display`),
    [valueId(gun, "shot_sequence_low")]: itemValue(gun, "shot_sequence_low", `Gun${gun}_ShotSequenceLow_Display`),
    [valueId(gun, "settling_seconds")]: itemValue(gun, "settling_seconds", `Gun${gun}_SettlingSeconds_Display`),
    [valueId(gun, "lockout_seconds")]: itemValue(gun, "lockout_seconds", `Gun${gun}_ReloadingSeconds_Display`),
    [ledId(gun, "ready")]: itemLed(gun, "ready", `Gun${gun}_Ready_LED`, colors.online),
    [ledId(gun, "on_target")]: itemLed(gun, "on_target", `Gun${gun}_OnTarget_LED`, colors.online),
    [ledId(gun, "settling")]: itemLed(gun, "settling", `Gun${gun}_Settling_LED`, colors.warning),
    [ledId(gun, "firing")]: itemLed(gun, "firing", `Gun${gun}_Firing_LED`, colors.fault),
    [ledId(gun, "lockout")]: itemLed(gun, "lockout", `Gun${gun}_Reloading_LED`, "#f97316ff"),
    [ledId(gun, "fault_active")]: itemLed(gun, "fault_active", `Gun${gun}_Fault_LED`, colors.fault),
    [ledId(gun, "target_accepted")]: itemLed(gun, "target_accepted", `Gun${gun}_TargetAccepted_LED`, colors.accent),
    [buttonId(gun)]: itemButton(gun)
  };

  const svg = [
    `<svg width="1280" height="720" xmlns="http://www.w3.org/2000/svg" xmlns:html="http://www.w3.org/1999/xhtml">`,
    "<g>",
    `<title>Stonehenge Gun ${gun} Auto HMI</title>`,
    rect("bg", 0, 0, 1280, 720, colors.body, colors.body, 0, 0),
    rect("topbar", 24, 10, 1232, 68, colors.header, colors.border, 8, 1.2),
    text("eyebrow", 48, 38, "AUTOMATIC OPERATION STATUS", 13, colors.accent, "900"),
    text("title", 48, 66, `Stonehenge Gun ${gun} Auto HMI`, 28, colors.text, "800"),
    rect("left_panel", 24, 116, 370, 548, colors.card, colors.border, 10, 1.4),
    text("left_title", 48, 150, "Gun Telemetry", 21, colors.text, "800"),
    text("state_lbl", 52, 197, "State", 13, colors.muted, "800"),
    valueSvg(gun, "state", 198, 200, 136),
    text("az_lbl", 52, 253, "Azimuth x10", 13, colors.muted, "800"),
    valueSvg(gun, "current_azimuth", 198, 256, 136),
    text("el_lbl", 52, 309, "Elevation x10", 13, colors.muted, "800"),
    valueSvg(gun, "current_elevation", 198, 312, 136),
    text("frag_lbl", 52, 365, "Active fragment", 13, colors.muted, "800"),
    valueSvg(gun, "active_fragment_low", 198, 368, 136),
    text("fault_lbl", 52, 421, "Fault code", 13, colors.muted, "800"),
    valueSvg(gun, "fault_code", 198, 424, 136),
    text("shot_lbl", 52, 477, "Shot sequence", 13, colors.muted, "800"),
    valueSvg(gun, "shot_sequence_low", 198, 480, 136),
    text("settle_lbl", 52, 533, "Settling seconds", 13, colors.muted, "800"),
    valueSvg(gun, "settling_seconds", 198, 536, 136),
    text("lockout_lbl", 52, 589, "Reloading seconds", 13, colors.muted, "800"),
    valueSvg(gun, "lockout_seconds", 198, 592, 136),
    rect("hmi_panel", 418, 116, 500, 548, colors.panel, colors.border, 10, 1.4),
    text("array_title", 448, 150, "Auto Cycle Overview", 21, colors.text, "800"),
    `<circle id="array_ring" cx="668" cy="390" r="156" fill="#0b1220ff" stroke="#475569ff" stroke-width="2"/>`,
    `<circle id="gun_core" cx="668" cy="390" r="72" fill="#111827ff" stroke="${colors.accent}" stroke-width="2"/>`,
    text("gun_core_num", 668, 384, `G${gun}`, 38, colors.text, "900", "middle"),
    text("gun_core_lbl", 668, 413, "AUTO", 16, colors.accent, "900", "middle"),
    `<line id="gun_${gun}_azimuth_pointer" x1="668" y1="390" x2="782" y2="294" stroke="${colors.accent}" stroke-width="5" stroke-linecap="round"/>`,
    `<circle id="gun_${gun}_azimuth_dot" cx="782" cy="294" r="14" fill="${colors.accent}" stroke="#bfdbfeff" stroke-width="3"/>`,
    rect("right_panel", 942, 116, 314, 548, colors.card, colors.border, 10, 1.4),
    text("status_title", 970, 150, "Cycle Status", 21, colors.text, "800"),
    ledSvg(gun, "ready", "Ready", 982, 205),
    ledSvg(gun, "on_target", "On Target", 982, 250),
    ledSvg(gun, "target_accepted", "Target Accepted", 982, 295),
    ledSvg(gun, "settling", "Settling", 982, 340),
    ledSvg(gun, "firing", "Firing", 982, 385),
    ledSvg(gun, "lockout", "Reloading", 982, 430),
    ledSvg(gun, "fault_active", "Fault Active", 982, 475),
    rect("reset_panel", 960, 512, 270, 112, colors.amberPanel, colors.amberBorder, 8, 1.4),
    text("reset_title", 982, 540, "Operator Reset", 16, "#fef3c7ff", "700"),
    resetButtonSvg(gun),
    "</g>",
    "</svg>"
  ].join("\n");

  return {
    id: `v_stonehenge_gun${gun}_auto`,
    name: `Gun ${gun} Auto HMI`,
    profile: {
      width: 1280,
      height: 720,
      bkcolor: colors.body,
      margin: 0,
      align: "topCenter",
      gridType: "fixed",
      viewRenderDelay: 0
    },
    items,
    variables: {},
    svgcontent: svg,
    type: "svg"
  };
}

function manualButtonSvg(gun, id, x, y, width, textValue, bg, color = "#ffffff", disabled = true) {
  const disabledAttr = disabled ? ` disabled="disabled"` : "";
  const opacity = disabled ? ".82" : "1";
  return [
    `<g data-name="Gun${gun}_${id}" id="HXB_manual_gun${gun}_${id}" type="svg-ext-html_button" fill="rgba(0,0,0,0)" stroke="rgba(0,0,0,0)" style="cursor:pointer;pointer-events:all;">`,
    `<rect id="HXB_manual_gun${gun}_${id}_rect" x="${x}" y="${y}" width="${width}" height="48" rx="8" fill="rgba(0,0,0,0.01)" stroke-width="0" style="pointer-events:all;"/>`,
    `<foreignObject id="HXB_manual_gun${gun}_${id}_fo" x="${x}" y="${y}" width="${width}" height="48" style="pointer-events:none;">`,
    `<button xmlns="http://www.w3.org/1999/xhtml" tabindex="-1"${disabledAttr} style="pointer-events:none;width:calc(100% - 6px);height:calc(100% - 6px);margin:3px;text-align:center;background-color:${bg};color:${color};border:1px solid #475569;border-radius:7px;font-size:13px;font-family:${fontStack};font-weight:700;letter-spacing:.02em;opacity:${opacity};">${textValue}</button>`,
    "</foreignObject>",
    "</g>"
  ].join("\n");
}

function manualInputSvg(id, x, y, width, placeholder, min, max) {
  return [
    `<g data-name="${id}" id="${id}" type="svg-ext-html_input" fill="#0b1220" stroke="#475569">`,
    `<rect x="${x}" y="${y}" width="${width}" height="38" rx="6" fill="#0b1220" stroke="#475569" stroke-width="1.2"/>`,
    `<foreignObject x="${x}" y="${y}" height="38" width="${width}">`,
    `<INPUT style="width:calc(100% - 8px);height:calc(100% - 8px);margin:4px;text-align:center;border:unset;border-radius:5px;font-size:13px;font-family:${fontStack};font-weight:600;background-color:#0b1220;color:#e5e7eb;" value="${placeholder}" type="number"/>`,
    `</foreignObject>`,
    `</g>`
  ].join("\n");
}

function inputSyncRuntimeSvg() {
  return "";
}

function hmiButtonSvg(gun, key, x, y, width, label, fireRequest = false) {
  const bg = fireRequest ? "#991b1b" : "#1e3a8a";
  const fg = fireRequest ? "#fee2e2" : "#dbeafe";
  const border = fireRequest ? "#ef4444" : "#60a5fa";
  const click = hmiClientClick(gun, key);
  const clickAttr = click ? ` onclick="${attr(click)}"` : "";
  return [
    `<g data-name="${key}" id="${hmiButtonId(gun, key)}" type="svg-ext-html_button" fill="rgba(0,0,0,0)" stroke="rgba(0,0,0,0)" style="cursor:pointer;pointer-events:all;"${clickAttr}>`,
    `<rect x="${x}" y="${y}" width="${width}" height="40" rx="7" fill="rgba(0,0,0,0.01)" stroke-width="0" style="pointer-events:all;"/>`,
    `<foreignObject x="${x}" y="${y}" height="40" width="${width}" style="pointer-events:none;">`,
    `<BUTTON tabindex="-1" style="pointer-events:none;width:calc(100% - 6px);height:calc(100% - 6px);margin:3px;text-align:center;background-color:${bg};color:${fg};border:1px solid ${border};border-radius:7px;font-size:13px;font-family:${fontStack};font-weight:700;">${label}</BUTTON>`,
    `</foreignObject>`,
    `</g>`
  ].join("\n");
}

function navButtonSvg(gun, kind, x, y, width, label, active = false) {
  const bg = active ? "#1d4ed8" : "#111827";
  const fg = active ? "#eff6ff" : "#bfdbfe";
  const border = active ? "#60a5fa" : "#374151";
  return [
    `<g data-name="nav_${kind}" id="${navButtonId(gun, kind)}" type="svg-ext-html_button" fill="rgba(0,0,0,0)" stroke="rgba(0,0,0,0)" style="cursor:pointer;pointer-events:all;">`,
    `<rect x="${x}" y="${y}" width="${width}" height="28" rx="6" fill="rgba(0,0,0,0.01)" stroke-width="0" style="pointer-events:all;"/>`,
    `<foreignObject x="${x}" y="${y}" height="28" width="${width}" style="pointer-events:none;">`,
    `<BUTTON tabindex="-1" style="pointer-events:none;width:calc(100% - 4px);height:calc(100% - 4px);margin:2px;text-align:center;background-color:${bg};color:${fg};border:1px solid ${border};border-radius:6px;font-size:11px;font-family:${fontStack};font-weight:700;">${label}</BUTTON>`,
    `</foreignObject>`,
    `</g>`
  ].join("\n");
}

function viewNavSvg(gun, activeKind) {
  const prev = gun === 1 ? 8 : gun - 1;
  const next = gun === 8 ? 1 : gun + 1;
  return [
    navButtonSvg(gun, "auto", 842, 42, 72, "AUTO", activeKind === "auto"),
    navButtonSvg(gun, "manual", 922, 42, 86, "MANUAL", activeKind === "manual"),
    navButtonSvg(gun, "admin", 1016, 42, 76, "ADMIN", activeKind === "admin"),
    navButtonSvg(prev, "auto", 1108, 42, 56, `G${prev}`),
    navButtonSvg(next, "auto", 1172, 42, 56, `G${next}`)
  ].join("\n");
}

function adminModeButtonsSvg(gun, x, y, width) {
  const buttonWidth = Math.floor((width - 8) / 2);
  const modes = [
    ["manual", "MANUAL"],
    ["auto", "AUTO"]
  ];
  const buttons = modes.map(([mode, label], index) => {
    const left = x + index * (buttonWidth + 8);
    return hmiButtonSvg(gun, `admin_mode_${mode}`, left, y, buttonWidth, label, false);
  }).join("");
  return buttons;
}

function adminElevationInputSvg(gun, x, y, width) {
  const inputId = hmiInputId(gun, "admin_elevation");
  return manualInputSvg(inputId, x, y, width, "50", 5, 85);
}

function adminOverrideKeysSvg(gun, x, y, width) {
  const rootId = adminOverrideKeysRootId(gun);
  const half = Math.floor((width - 12) / 2);
  return [
    `<g id="${rootId}" font-family="${fontStack}">`,
    text(`${rootId}_title`, x, y + 10, "Override Keys", 12, colors.muted, "700"),
    `<text data-status="true" x="${x + width}" y="${y + 10}" fill="#fbbf24" font-size="12" font-family="${fontStack}" font-weight="700" text-anchor="end">KEYED OVERRIDES</text>`,
    `<text x="${x}" y="${y + 43}" fill="${colors.muted}" font-size="11" font-family="${fontStack}" font-weight="700">ADMIN OVERRIDE</text>`,
    hmiLocalValueSvg(gun, "admin_override_state", x, y + 67, half, 72),
    hmiSetButtonSvg(gun, "admin_override", false, x, y + 78, Math.floor((half - 8) / 2), 32, "LOCK", false),
    hmiSetButtonSvg(gun, "admin_override", true, x + Math.floor((half - 8) / 2) + 8, y + 78, Math.floor((half - 8) / 2), 32, "UNLOCK", true),
    `<text x="${x + half + 12}" y="${y + 43}" fill="${colors.muted}" font-size="11" font-family="${fontStack}" font-weight="700">EXT PERMISSIVE</text>`,
    hmiLocalValueSvg(gun, "extended_permissive_state", x + half + 12, y + 67, half, 72),
    hmiSetButtonSvg(gun, "extended_permissive", false, x + half + 12, y + 78, Math.floor((half - 8) / 2), 32, "LOCK", false),
    hmiSetButtonSvg(gun, "extended_permissive", true, x + half + 12 + Math.floor((half - 8) / 2) + 8, y + 78, Math.floor((half - 8) / 2), 32, "UNLOCK", true),
    `<g>`,
    `<rect x="${x}" y="${y + 118}" width="${width}" height="26" rx="6" fill="#3f0505" stroke="#ef4444" stroke-width="1"/>`,
    `<text x="${x + width / 2}" y="${y + 136}" fill="#fecaca" font-size="12" font-family="${fontStack}" font-weight="700" text-anchor="middle">OVERRIDE SWITCHES ENABLE DANGER RANGE</text>`,
    `</g>`,
    `</g>`
  ].join("\n");
}

function hmiSetButtonSvg(gun, key, value, x, y, width, height, label, danger = false) {
  const bg = danger ? "#7f1d1d" : "#1f2937";
  const fg = danger ? "#fee2e2" : "#e5e7eb";
  const border = danger ? "#ef4444" : "#475569";
  const stateKey = `g${gun}_${key}`;
  const valueText = value ? "UNLOCKED" : "LOCKED";
  const statusColor = value ? "#86efac" : "#dbeafe";
  const click = [
    "event.stopPropagation();",
    "event.preventDefault();",
    "const state=window.__stonehengeAdminState=window.__stonehengeAdminState||{};",
    `state['${stateKey}']=${value ? "true" : "false"};`,
    `const el=document.querySelector('#${hmiLocalValueId(gun, `${key}_state`)}_text');`,
    `if(el){el.textContent='${valueText}';el.setAttribute('fill','${statusColor}');}`,
    "return false;"
  ].join("");
  return [
    `<g data-name="${key}_${value ? "unlock" : "lock"}" id="${hmiSetButtonId(gun, key, value)}" type="svg-ext-html_button" fill="rgba(0,0,0,0)" stroke="rgba(0,0,0,0)" style="cursor:pointer;pointer-events:all;" onclick="${attr(click)}">`,
    `<rect x="${x}" y="${y}" width="${width}" height="${height}" rx="7" fill="rgba(0,0,0,0.01)" stroke-width="0" style="pointer-events:all;"/>`,
    `<foreignObject x="${x}" y="${y}" width="${width}" height="${height}" style="pointer-events:none;">`,
    `<BUTTON tabindex="-1" style="pointer-events:none;width:calc(100% - 4px);height:calc(100% - 4px);margin:2px;text-align:center;background-color:${bg};color:${fg};border:1px solid ${border};border-radius:7px;font-size:10px;font-family:${fontStack};font-weight:700;">${label}</BUTTON>`,
    `</foreignObject>`,
    `</g>`
  ].join("\n");
}

function hmiSwitchSvg(gun, key, x, y, width, height) {
  return [
    `<g data-name="${key}" id="${hmiSwitchId(gun, key)}" type="svg-ext-html_switch" fill="rgba(0,0,0,0)" stroke="rgba(0,0,0,0)">`,
    `<rect stroke-width="0" x="${x}" y="${y}" width="${width}" height="${height}" fill="rgba(0,0,0,0)"/>`,
    `<foreignObject x="${x}" y="${y}" width="${width}" height="${height}">`,
    `<LABEL style="width:calc(100% - 6px);height:calc(100% - 6px);text-align:center;background-color:#0b1220;color:#e5e7eb;margin:3px;border-radius:6px;" class="md-switch">`,
    `<NGX-SWITCH class="ng-star-inserted">`,
    `<LABEL style="border-radius:6px;" class="md-switch">`,
    `<DIV style="background-color:#334155;opacity:1;pointer-events:auto;border:1px solid #475569;border-radius:6px;" class="toggle-button">`,
    `<INPUT type="checkbox"/>`,
    `<DIV style="text-align:center;line-height:28px;font-size:11px;background-color:#94a3b8;color:#e5e7eb;font-family:${fontStack};border-radius:6px;" class="inner-circle"></DIV>`,
    `</DIV>`,
    `</LABEL>`,
    `</NGX-SWITCH>`,
    `</LABEL>`,
    `</foreignObject>`,
    `</g>`
  ].join("\n");
}

function adminOverrideKeysRootId(gun) {
  return `admin_override_keys_gun${gun}`;
}

function buildManualView(gun) {
  const limits = gunLimits[gun];
  const items = {
    [valueId(gun, "state")]: itemValue(gun, "state", `Gun${gun}_Manual_State_Display`),
    [valueId(gun, "current_azimuth")]: itemValue(gun, "current_azimuth", `Gun${gun}_Manual_CurrentAzimuth_Display`),
    [valueId(gun, "current_elevation")]: itemValue(gun, "current_elevation", `Gun${gun}_Manual_CurrentElevation_Display`),
    [valueId(gun, "fault_code")]: itemValue(gun, "fault_code", `Gun${gun}_Manual_FaultCode_Display`),
    [valueId(gun, "lockout_seconds")]: itemValue(gun, "lockout_seconds", `Gun${gun}_Manual_ReloadingSeconds_Display`),
    [ledId(gun, "ready")]: itemLed(gun, "ready", `Gun${gun}_Manual_Ready_LED`, colors.online),
    [ledId(gun, "on_target")]: itemLed(gun, "on_target", `Gun${gun}_Manual_OnTarget_LED`, colors.online),
    [ledId(gun, "firing")]: itemLed(gun, "firing", `Gun${gun}_Manual_Firing_LED`, colors.fault),
    [ledId(gun, "lockout")]: itemLed(gun, "lockout", `Gun${gun}_Manual_Reloading_LED`, "#f97316ff"),
    [ledId(gun, "fault_active")]: itemLed(gun, "fault_active", `Gun${gun}_Manual_Fault_LED`, colors.fault),
    [hmiInputId(gun, "manual_azimuth")]: itemHmiInput(gun, "manual_azimuth", `Gun${gun}_Manual_TargetAzimuth_Input`, limits.minAzimuth, limits.maxAzimuth),
    [hmiInputId(gun, "manual_elevation")]: itemHmiInput(gun, "manual_elevation", `Gun${gun}_Manual_TargetElevation_Input`, limits.minElevation, limits.maxElevation),
    [hmiButtonId(gun, "manual_stage")]: itemHmiButton(gun, "manual_stage", "STAGE TARGET")
  };

  const svg = [
    `<svg width="1280" height="720" xmlns="http://www.w3.org/2000/svg" xmlns:html="http://www.w3.org/1999/xhtml">`,
    "<g>",
    `<title>Stonehenge Gun ${gun} Manual HMI</title>`,
    rect("bg", 0, 0, 1280, 720, colors.body, colors.body, 0, 0),
    inputSyncRuntimeSvg(),
    rect("topbar", 24, 10, 1232, 68, colors.header, colors.border, 8, 1.2),
    text("eyebrow", 48, 38, "MANUAL OPERATOR WORKSPACE", 13, colors.accent, "900"),
    text("title", 48, 66, `Stonehenge Gun ${gun} Manual HMI`, 28, colors.text, "800"),
    rect("hmi_panel", 24, 116, 790, 548, colors.panel, colors.border, 10, 1.4),
    text("hmi_title", 54, 150, "Manual HMI View", 21, colors.text, "800"),
    `<circle id="manual_ring" cx="420" cy="376" r="154" fill="#0b1220ff" stroke="#475569ff" stroke-width="2"/>`,
    `<circle id="manual_core" cx="420" cy="376" r="72" fill="#111827ff" stroke="${colors.accent}" stroke-width="2"/>`,
    text("gun_core_num", 420, 370, `G${gun}`, 38, colors.text, "900", "middle"),
    text("gun_core_lbl", 420, 399, "MANUAL", 16, colors.accent, "900", "middle"),
    text("gun_core_scale", 420, 424, "CMD", 12, colors.muted, "700", "middle"),
    text("state_lbl", 590, 218, "State", 13, colors.muted, "700"),
    valueSvg(gun, "state", 704, 221, 92),
    text("az_lbl", 590, 274, "Azimuth x10", 13, colors.muted, "700"),
    valueSvg(gun, "current_azimuth", 704, 277, 92),
    text("el_lbl", 590, 330, "Elevation x10", 13, colors.muted, "700"),
    valueSvg(gun, "current_elevation", 704, 333, 92),
    text("fault_lbl", 590, 386, "Fault code", 13, colors.muted, "700"),
    valueSvg(gun, "fault_code", 704, 389, 92),
    text("lock_lbl", 590, 442, "Reload sec", 13, colors.muted, "700"),
    valueSvg(gun, "lockout_seconds", 704, 445, 92),
    ledSvg(gun, "ready", "Ready", 74, 585),
    ledSvg(gun, "on_target", "On Target", 220, 585),
    ledSvg(gun, "firing", "Firing", 394, 585),
    ledSvg(gun, "lockout", "Reloading", 540, 585),
    ledSvg(gun, "fault_active", "Fault", 690, 585),
    rect("setup_panel", 838, 116, 418, 548, "#18120fff", colors.amberBorder, 10, 1.6),
    text("setup_eye", 868, 150, "MANUAL SETUP", 13, colors.accent, "900"),
    text("setup_title", 868, 180, "Manual Setup", 24, colors.text, "700"),
    rect("coord_stage", 868, 218, 358, 352, "#0b1220ff", "#475569ff", 8, 1.2),
    text("coord_title", 890, 258, "Target Coordinates", 18, colors.text, "700"),
    text("coord_scale", 1204, 258, "degrees", 12, colors.accent, "700", "end"),
    text("az_input_lbl", 890, 322, "Azimuth", 12, colors.muted, "700"),
    manualInputSvg(hmiInputId(gun, "manual_azimuth"), 1006, 298, 196, `${limits.minAzimuth}`, limits.minAzimuth, limits.maxAzimuth),
    text("el_input_lbl", 890, 388, "Elevation", 12, colors.muted, "700"),
    manualInputSvg(hmiInputId(gun, "manual_elevation"), 1006, 364, 196, "50", limits.minElevation, limits.maxElevation),
    hmiButtonSvg(gun, "manual_stage", 1006, 448, 196, "STAGE TARGET"),
    "</g>",
    "</svg>"
  ].join("\n");

  return {
    id: `v_stonehenge_gun${gun}_manual`,
    name: `Gun ${gun} Manual HMI`,
    profile: {
      width: 1280,
      height: 720,
      bkcolor: colors.body,
      margin: 0,
      align: "topCenter",
      gridType: "fixed",
      viewRenderDelay: 0
    },
    items,
    variables: {},
    svgcontent: svg,
    type: "svg"
  };
}

function buildAdminManualView(gun) {
  const limits = gunLimits[gun];
  const items = {
    [valueId(gun, "state")]: itemValue(gun, "state", `Gun${gun}_AdminManual_State_Display`),
    [valueId(gun, "current_azimuth")]: itemValue(gun, "current_azimuth", `Gun${gun}_AdminManual_CurrentAzimuth_Display`),
    [valueId(gun, "current_elevation")]: itemValue(gun, "current_elevation", `Gun${gun}_AdminManual_CurrentElevation_Display`),
    [valueId(gun, "fault_code")]: itemValue(gun, "fault_code", `Gun${gun}_AdminManual_FaultCode_Display`),
    [valueId(gun, "lockout_seconds")]: itemValue(gun, "lockout_seconds", `Gun${gun}_AdminManual_ReloadingSeconds_Display`),
    [ledId(gun, "ready")]: itemLed(gun, "ready", `Gun${gun}_AdminManual_Ready_LED`, colors.online),
    [ledId(gun, "on_target")]: itemLed(gun, "on_target", `Gun${gun}_AdminManual_OnTarget_LED`, colors.online),
    [ledId(gun, "firing")]: itemLed(gun, "firing", `Gun${gun}_AdminManual_Firing_LED`, colors.fault),
    [ledId(gun, "lockout")]: itemLed(gun, "lockout", `Gun${gun}_AdminManual_Reloading_LED`, "#f97316ff"),
    [ledId(gun, "fault_active")]: itemLed(gun, "fault_active", `Gun${gun}_AdminManual_Fault_LED`, colors.fault),
    [manualModeButtonId(gun)]: itemManualModeButton(gun),
    [hmiInputId(gun, "admin_azimuth")]: itemHmiInput(gun, "admin_azimuth", `Gun${gun}_Admin_TargetAzimuth_Input`, limits.minAzimuth, limits.maxAzimuth),
    [hmiInputId(gun, "admin_elevation")]: itemHmiInput(gun, "admin_elevation", `Gun${gun}_Admin_TargetElevation_Input`, -30, 90),
    [hmiSetButtonId(gun, "admin_override", false)]: itemHmiSetButton(gun, "admin_override", false, "LOCK"),
    [hmiSetButtonId(gun, "admin_override", true)]: itemHmiSetButton(gun, "admin_override", true, "UNLOCK"),
    [hmiSetButtonId(gun, "extended_permissive", false)]: itemHmiSetButton(gun, "extended_permissive", false, "LOCK"),
    [hmiSetButtonId(gun, "extended_permissive", true)]: itemHmiSetButton(gun, "extended_permissive", true, "UNLOCK"),
    [hmiLocalValueId(gun, "admin_override_state")]: itemHmiLocalValue(gun, "admin_override_state", `Gun${gun}_AdminOverride_State`),
    [hmiLocalValueId(gun, "extended_permissive_state")]: itemHmiLocalValue(gun, "extended_permissive_state", `Gun${gun}_ExtendedPermissive_State`),
    [hmiLocalValueId(gun, "admin_mode_state")]: itemHmiLocalValue(gun, "admin_mode_state", `Gun${gun}_AdminMode_State`),
    [hmiLocalValueId(gun, "admin_action_state")]: itemHmiLocalValue(gun, "admin_action_state", `Gun${gun}_AdminAction_State`),
    [hmiButtonId(gun, "admin_stage")]: itemHmiButton(gun, "admin_stage", "STAGE"),
    [hmiButtonId(gun, "admin_fire")]: itemHmiButton(gun, "admin_fire", "FIRE"),
    [hmiButtonId(gun, "admin_mode_manual")]: itemHmiButton(gun, "admin_mode_manual", "MANUAL"),
    [hmiButtonId(gun, "admin_mode_auto")]: itemHmiButton(gun, "admin_mode_auto", "AUTO")
  };

  const svg = [
    `<svg width="1280" height="720" xmlns="http://www.w3.org/2000/svg" xmlns:html="http://www.w3.org/1999/xhtml">`,
    "<g>",
    `<title>Stonehenge Gun ${gun} Admin Manual HMI</title>`,
    rect("bg", 0, 0, 1280, 720, colors.body, colors.body, 0, 0),
    inputSyncRuntimeSvg(),
    rect("topbar", 24, 10, 1232, 68, colors.header, colors.amberBorder, 8, 1.4),
    text("eyebrow", 48, 38, "ADMIN MANUAL WORKSPACE", 13, colors.warning, "700"),
    text("title", 48, 66, `Stonehenge Gun ${gun} Admin Manual HMI`, 28, colors.text, "700"),
    rect("hmi_panel", 24, 116, 790, 548, colors.panel, colors.border, 10, 1.4),
    text("hmi_title", 54, 150, "Admin Manual HMI", 21, colors.text, "700"),
    `<circle id="manual_ring" cx="420" cy="376" r="154" fill="#0b1220ff" stroke="#475569ff" stroke-width="2"/>`,
    `<circle id="manual_core" cx="420" cy="376" r="72" fill="#111827ff" stroke="${colors.warning}" stroke-width="2"/>`,
    text("gun_core_num", 420, 370, `G${gun}`, 38, colors.text, "700", "middle"),
    text("gun_core_lbl", 420, 399, "ADMIN", 16, colors.warning, "700", "middle"),
    text("gun_core_scale", 420, 424, "CMD", 12, colors.muted, "700", "middle"),
    text("state_lbl", 590, 218, "State", 13, colors.muted, "700"),
    valueSvg(gun, "state", 704, 221, 92),
    text("az_lbl", 590, 274, "Azimuth x10", 13, colors.muted, "700"),
    valueSvg(gun, "current_azimuth", 704, 277, 92),
    text("el_lbl", 590, 330, "Elevation x10", 13, colors.muted, "700"),
    valueSvg(gun, "current_elevation", 704, 333, 92),
    text("fault_lbl", 590, 386, "Fault code", 13, colors.muted, "700"),
    valueSvg(gun, "fault_code", 704, 389, 92),
    text("lock_lbl", 590, 442, "Reload sec", 13, colors.muted, "700"),
    valueSvg(gun, "lockout_seconds", 704, 445, 92),
    ledSvg(gun, "ready", "Ready", 74, 585),
    ledSvg(gun, "on_target", "On Target", 220, 585),
    ledSvg(gun, "firing", "Firing", 394, 585),
    ledSvg(gun, "lockout", "Reloading", 540, 585),
    ledSvg(gun, "fault_active", "Fault", 690, 585),
    rect("setup_panel", 838, 116, 418, 548, "#18120fff", colors.amberBorder, 10, 1.6),
    text("setup_eye", 868, 150, "ADMIN SETUP", 13, colors.warning, "700"),
    text("setup_title", 868, 180, "Admin Controls", 24, colors.text, "700"),
    rect("mode_stage", 868, 204, 358, 84, "#0b1220ff", "#78350fff", 8, 1.2),
    text("mode_title", 890, 234, "Gun Mode", 14, colors.muted, "700"),
    adminModeButtonsSvg(gun, 890, 240, 314),
    hmiLocalValueSvg(gun, "admin_mode_state", 1130, 234, 80),
    rect("key_stage", 868, 296, 358, 158, "#09090bff", "#7f1d1dff", 8, 1.2),
    adminOverrideKeysSvg(gun, 884, 306, 326),
    rect("coord_stage", 868, 464, 358, 120, "#0b1220ff", colors.amberBorder, 8, 1.2),
    text("coord_title", 890, 496, "Target Coordinates", 18, colors.text, "700"),
    text("coord_scale", 1204, 496, "degrees", 12, colors.warning, "700", "end"),
    text("az_input_lbl", 890, 536, "Azimuth", 12, colors.muted, "700"),
    manualInputSvg(hmiInputId(gun, "admin_azimuth"), 1006, 512, 196, `${limits.minAzimuth}`, limits.minAzimuth, limits.maxAzimuth),
    text("el_input_lbl", 890, 576, "Elevation", 12, colors.muted, "700"),
    adminElevationInputSvg(gun, 1006, 552, 196),
    rect("fire_stage", 868, 596, 358, 66, "#09090bff", "#7f1d1dff", 8, 1.2),
    hmiButtonSvg(gun, "admin_stage", 884, 598, 154, "STAGE"),
    hmiButtonSvg(gun, "admin_fire", 1050, 598, 154, "FIRE", true),
    hmiLocalValueSvg(gun, "admin_action_state", 884, 654, 320),
    "</g>",
    "</svg>"
  ].join("\n");

  return {
    id: adminManualViewId(gun),
    name: `Gun ${gun} Admin Manual HMI`,
    profile: {
      width: 1280,
      height: 720,
      bkcolor: colors.body,
      margin: 0,
      align: "topCenter",
      gridType: "fixed",
      viewRenderDelay: 0
    },
    items,
    variables: {},
    svgcontent: svg,
    type: "svg"
  };
}

async function main() {
  if (!fs.existsSync(dbPath)) {
    throw new Error(`FUXA project DB not found: ${dbPath}`);
  }

  const backupPath = `${dbPath}.bak-${new Date().toISOString().replace(/[:.]/g, "-")}`;
  fs.copyFileSync(dbPath, backupPath);
  console.log(`Backed up ${dbPath} to ${backupPath}`);

  const db = new sqlite3.Database(dbPath);
  try {
    await sqlRun(db, "BEGIN TRANSACTION");

    const serverRow = await sqlGet(db, "SELECT value FROM devices WHERE name = ?", ["0"]);
    const existingServer = serverRow ? JSON.parse(serverRow.value) : {};
    const fuxaServer = buildFuxaServerDevice(existingServer);
    await sqlRun(
      db,
      "INSERT OR REPLACE INTO devices (name, value) VALUES (?, ?)",
      ["0", JSON.stringify(fuxaServer)]
    );

    for (let gun = 1; gun <= 8; gun += 1) {
      const device = buildGunDevice(gun);
      const view = buildAutoView(gun);
      const manualView = buildManualView(gun);
      const adminManualView = buildAdminManualView(gun);
      await sqlRun(
        db,
        "INSERT OR REPLACE INTO devices (name, value) VALUES (?, ?)",
        [device.id, JSON.stringify(device)]
      );
      await sqlRun(
        db,
        "INSERT OR REPLACE INTO views (name, value) VALUES (?, ?)",
        [view.id, JSON.stringify(view)]
      );
      await sqlRun(
        db,
        "INSERT OR REPLACE INTO views (name, value) VALUES (?, ?)",
        [manualView.id, JSON.stringify(manualView)]
      );
      await sqlRun(
        db,
        "INSERT OR REPLACE INTO views (name, value) VALUES (?, ?)",
        [adminManualView.id, JSON.stringify(adminManualView)]
      );
      for (const script of buildScripts(gun)) {
        await sqlRun(
          db,
          "INSERT OR REPLACE INTO scripts (name, value) VALUES (?, ?)",
          [script.id, JSON.stringify(script)]
        );
      }
      for (const scriptId of retiredDirectMiddlewareScriptIds(gun)) {
        await sqlRun(db, "DELETE FROM scripts WHERE name = ?", [scriptId]);
      }
    }

    const layout = {
      autoresize: false,
      start: "v_stonehenge_gun1_auto",
      navigation: {
        mode: "over",
        type: "block",
        bkcolor: "#111827",
        fgcolor: "#e5e7eb",
        logo: false
      },
      header: {
        bkcolor: "#111827",
        fgcolor: "#e5e7eb",
        height: 46,
        buttonHeight: 36,
        fontSize: 13,
        itemsAnchor: "left"
      },
      showdev: true,
      inputdialog: "false",
      hidenavigation: false,
      theme: "",
      loginonstart: false,
      loginoverlaycolor: "none",
      show_connection_error: true,
      customStyles: ""
    };

    await sqlRun(
      db,
      "INSERT OR REPLACE INTO general (name, value) VALUES (?, ?)",
      ["layout", JSON.stringify(layout)]
    );
    await sqlRun(
      db,
      "INSERT OR REPLACE INTO general (name, value) VALUES (?, ?)",
      ["name", JSON.stringify("Stonehenge-Gun-Auto-HMI-Status-Workspace")]
    );

    await sqlRun(db, "COMMIT");
    console.log("Installed Stonehenge Gun 1-8 Auto, Manual, and Admin Manual HMI views.");
  } catch (err) {
    await sqlRun(db, "ROLLBACK").catch(() => {});
    throw err;
  } finally {
    db.close();
  }
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});
