const assert = require("node:assert/strict");
const fs = require("node:fs");
const test = require("node:test");
const vm = require("node:vm");

class Element {
  constructor() {
    this.children = [];
    this.className = "";
    this.style = {};
    this.title = "";
    this._innerHTML = "";
  }

  set innerHTML(value) {
    this._innerHTML = String(value);
    if (value === "") this.children = [];
  }

  get innerHTML() {
    return this._innerHTML;
  }

  appendChild(child) {
    this.children.push(child);
  }
}

function loadDashboard() {
  const elements = {};
  const context = {
    document: {
      createElement: () => new Element(),
      getElementById: (id) => (elements[id] ||= new Element()),
    },
    fetch: () => new Promise(() => {}),
    setInterval: () => {},
  };
  vm.createContext(context);
  vm.runInContext(fs.readFileSync(`${__dirname}/static/app.js`, "utf8"), context);
  return { context, elements };
}

test("dashboard encodes API text before inserting HTML", () => {
  const { context, elements } = loadDashboard();
  const attack = `<img src=x onerror="alert(1)">&'`;
  const encoded = "&lt;img src=x onerror=&quot;alert(1)&quot;&gt;&amp;&#39;";

  context.renderGuns([{
    id: attack,
    state: attack,
    mode: attack,
    assigned_fragment_id: attack,
    safety_permissive: attack,
    safety_fault_reason: attack,
  }]);
  context.renderFragments([{
    id: attack,
    priority: attack,
    terrain_type: attack,
    strategic_zone: attack,
    status: attack,
    assigned_gun_id: attack,
  }]);
  context.renderMap([{
    id: attack,
    priority: attack,
    status: attack,
    terrain_type: attack,
    strategic_zone: attack,
    map_x: 50,
    map_y: 50,
  }]);
  context.renderEvents([{
    severity: attack,
    event_type: attack,
    gun_id: attack,
    fragment_id: attack,
    message: `duplicate external_id ${attack}`,
  }]);

  for (const id of ["guns", "fragments", "markerLayer", "events"]) {
    const html = elements[id].children.map((child) => child.innerHTML).join("");
    assert.ok(html.includes(encoded), `${id} did not encode attacker-controlled text`);
    assert.ok(!html.includes(attack), `${id} retained attacker-controlled HTML`);
  }
  assert.match(context.cssToken(attack), /^[a-z0-9-]*$/);
  assert.ok(elements.markerLayer.children[0].title.includes(attack));
});

test("dashboard preserves ordinary fragment rendering", () => {
  const { context, elements } = loadDashboard();
  context.renderGuns([{ safety_permissive: false }]);
  context.renderFragments([{
    id: "USEA-0001",
    priority: "HIGH",
    azimuth: 247.3,
    elevation: 41.8,
    terrain_type: "LAND",
    strategic_zone: "SAN_SALVACION",
    status: "ASSIGNED",
    assigned_gun_id: 6,
  }]);

  assert.match(elements.guns.children[0].innerHTML, /class="safe-false">false<\/span>/);
  const html = elements.fragments.children[0].innerHTML;
  assert.match(html, /USEA-0001/);
  assert.match(html, /priority-high/);
  assert.match(html, /247\.3 \/ 41\.8|247\.3/);
  assert.match(html, /LAND/);
  assert.match(html, /SAN_SALVACION/);
  assert.match(html, /status-assigned/);
});
