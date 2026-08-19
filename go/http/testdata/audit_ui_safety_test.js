const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

function loadHelpers(filename) {
  const sourcePath = path.resolve(__dirname, "../../../resources/public/js", filename);
  const source = fs.readFileSync(sourcePath, "utf8");
  const module = {exports: {}};
  const document = {};
  const sandbox = {
    document,
    module,
    exports: module.exports,
    encodeURIComponent,
    $() {
      return {ready() { return this; }};
    },
  };
  vm.runInNewContext(source, sandbox, {filename: sourcePath});
  return module.exports;
}

function escapeText(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

class Element {
  constructor(tag) {
    this.tag = tag;
    this.attributes = {};
    this.children = [];
    this.classes = [];
  }

  addClass(value) {
    this.classes.push(...String(value).split(/\s+/).filter(Boolean));
    return this;
  }

  attr(name, value) {
    if (typeof name === "object") {
      Object.entries(name).forEach(([key, entry]) => { this.attributes[key] = entry; });
      return this;
    }
    if (arguments.length === 1) return this.attributes[name];
    this.attributes[name] = value;
    return this;
  }

  append(...children) {
    this.children.push(...children.flat().filter(child => child !== null && child !== undefined));
    return this;
  }

  prepend(...children) {
    this.children.unshift(...children.flat().filter(child => child !== null && child !== undefined));
    return this;
  }

  text(value) {
    this.children = [String(value ?? "")];
    return this;
  }

  serialize() {
    const attributes = {...this.attributes};
    if (this.classes.length) attributes.class = this.classes.join(" ");
    const renderedAttributes = Object.entries(attributes)
      .map(([name, value]) => ` ${name}="${escapeText(value)}"`)
      .join("");
    const children = this.children
      .map(child => child instanceof Element ? child.serialize() : escapeText(child))
      .join("");
    return `<${this.tag}${renderedAttributes}>${children}</${this.tag}>`;
  }
}

function fakeJQuery(markup) {
  const match = /^<([a-z0-9]+)/i.exec(markup);
  if (!match) throw new Error(`unsupported fake jQuery selector: ${markup}`);
  return new Element(match[1].toLowerCase());
}

const hostile = `<img src=x onerror="globalThis.pwned=1">'&`;
const appUrl = value => `/prefix${value}`;
const instanceTitle = (hostname, port) => `${hostname}:${port}`;

function recoveryFixture() {
  return {
    Id: hostile,
    UID: `uid/${hostile}`,
    Acknowledged: true,
    AcknowledgedBy: hostile,
    AcknowledgedAt: hostile,
    AcknowledgedComment: hostile,
    LastDetectionId: `detect/${hostile}`,
    ProcessingNodeHostname: hostile,
    LostReplicas: [{Hostname: hostile, Port: 3306}],
    ParticipatingInstanceKeys: [{Hostname: hostile, Port: 3307}],
    AllErrors: [hostile],
    AnalysisEntry: {
      Analysis: hostile,
      CountReplicas: 1,
      Replicas: [{Hostname: hostile, Port: 3308}],
      AnalyzedInstanceKey: {Hostname: hostile, Port: 3306},
      ClusterDetails: {ClusterName: `cluster/${hostile}`, ClusterAlias: `alias/${hostile}`},
    },
    SuccessorKey: {Hostname: hostile, Port: 3309},
    RecoveryStartTimestamp: hostile,
    RecoveryEndTimestamp: hostile,
    IsSuccessful: true,
  };
}

test("one recovery on a list route remains list mode", function() {
  const helpers = loadHelpers("audit-recovery.js");
  assert.equal(typeof helpers.resolveRecoveryViewState, "function");

  assert.equal(helpers.resolveRecoveryViewState([recoveryFixture()], 0, "", false), "list");
});

test("id and uid recovery routes remain detail mode", function() {
  const helpers = loadHelpers("audit-recovery.js");
  assert.equal(typeof helpers.resolveRecoveryViewState, "function");

  assert.equal(helpers.resolveRecoveryViewState([recoveryFixture()], 7, "", false), "detail");
  assert.equal(helpers.resolveRecoveryViewState([recoveryFixture()], 0, "recovery-uid", false), "detail");
  assert.equal(helpers.resolveRecoveryViewState([], 7, "", false), "empty");
  assert.equal(helpers.resolveRecoveryViewState([recoveryFixture()], 7, "", true), "unavailable");
});

test("recovery acknowledgement and detail metadata keep hostile API text inert", function() {
  const helpers = loadHelpers("audit-recovery.js");
  assert.equal(typeof helpers.buildRecoveryAcknowledgement, "function");
  assert.equal(typeof helpers.buildRecoveryAuditInfo, "function");

  const audit = recoveryFixture();
  const acknowledgement = helpers.buildRecoveryAcknowledgement(fakeJQuery, audit).serialize();
  const metadata = helpers.buildRecoveryAuditInfo(fakeJQuery, audit, appUrl, instanceTitle).serialize();
  for (const rendered of [acknowledgement, metadata]) {
    assert.doesNotMatch(rendered, /<img\b/);
    assert.match(rendered, /&lt;img/);
  }
  assert.match(metadata, /detect%2F%3Cimg%20src%3Dx%20onerror%3D%22globalThis\.pwned%3D1%22%3E&#39;%26/);
});

test("recovery audit links use text nodes and encoded route segments", function() {
  const helpers = loadHelpers("audit-recovery.js");
  assert.equal(typeof helpers.buildRecoveryAuditLink, "function");

  const link = helpers.buildRecoveryAuditLink(fakeJQuery, hostile, "/web/cluster/alias/", `alias/${hostile}`, appUrl).serialize();
  assert.doesNotMatch(link, /<img\b/);
  assert.match(link, /href="\/prefix\/web\/cluster\/alias\/alias%2F%3Cimg/);
  assert.match(link, /&lt;img/);
});

function detectionFixture() {
  return {
    Id: hostile,
    RelatedRecoveryId: `recovery/${hostile}`,
    RecoveryStartTimestamp: hostile,
    ProcessingNodeHostname: hostile,
    AnalysisEntry: {
      Analysis: hostile,
      CountReplicas: 1,
      Replicas: [{Hostname: hostile, Port: 3306}],
      AnalyzedInstanceKey: {Hostname: hostile, Port: 3306},
      ClusterDetails: {ClusterName: `cluster/${hostile}`, ClusterAlias: `alias/${hostile}`},
    },
  };
}

test("failure detection metadata keeps host and changelog text inert", function() {
  const helpers = loadHelpers("audit-failure-detection.js");
  assert.equal(typeof helpers.buildFailureDetectionMoreInfo, "function");

  const audit = detectionFixture();
  const changelog = [`2026-01-01;${hostile}`];
  const metadata = helpers.buildFailureDetectionMoreInfo(fakeJQuery, audit, changelog, appUrl, instanceTitle).serialize();
  assert.doesNotMatch(metadata, /<img\b/);
  assert.match(metadata, /&lt;img/);
  assert.match(metadata, /recovery%2F%3Cimg%20src%3Dx%20onerror%3D%22globalThis\.pwned%3D1%22%3E&#39;%26/);
});

test("failure detection links encode hostile host and cluster route segments", function() {
  const helpers = loadHelpers("audit-failure-detection.js");
  assert.equal(typeof helpers.buildFailureDetectionLink, "function");

  const link = helpers.buildFailureDetectionLink(fakeJQuery, hostile, "/web/search/", `host/${hostile}`, appUrl).serialize();
  assert.doesNotMatch(link, /<img\b/);
  assert.match(link, /href="\/prefix\/web\/search\/host%2F%3Cimg/);
  assert.match(link, /&lt;img/);
});

test("audit request and pager route helpers encode reserved path characters", function() {
  const recoveryHelpers = loadHelpers("audit-recovery.js");
  const detectionHelpers = loadHelpers("audit-failure-detection.js");
  assert.equal(typeof recoveryHelpers.recoveryRouteSegment, "function");
  assert.equal(typeof detectionHelpers.failureDetectionRouteSegment, "function");

  assert.equal(recoveryHelpers.recoveryRouteSegment("cluster/east ?"), "cluster%2Feast%20%3F");
  assert.equal(detectionHelpers.failureDetectionRouteSegment("host/west ?"), "host%2Fwest%20%3F");
});

test("failure detection pager preserves exactly one URL prefix", function() {
  const helpers = loadHelpers("audit-failure-detection.js");
  assert.equal(typeof helpers.failureDetectionPagerUrl, "function");

  const baseWebUri = "/proxy/web/audit-failure-detection/";
  assert.equal(helpers.failureDetectionPagerUrl(baseWebUri, 2), "/proxy/web/audit-failure-detection/2");
  assert.equal(helpers.failureDetectionPagerUrl(baseWebUri, 4), "/proxy/web/audit-failure-detection/4");
});
