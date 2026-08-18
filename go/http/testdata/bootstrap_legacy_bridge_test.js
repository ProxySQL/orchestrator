const assert = require('node:assert/strict');
const test = require('node:test');
const bridge = require('../../../resources/public/js/bootstrap-legacy-bridge.js');

function fakeElement(attributes) {
  const values = new Map(Object.entries(attributes || {}));
  return {
    hasAttribute: name => values.has(name),
    getAttribute: name => values.get(name) ?? null,
    setAttribute: (name, value) => values.set(name, value),
    closest: () => null
  };
}

function fakeDocument(elements) {
  const listeners = new Map();
  return {
    querySelectorAll: selector => elements.filter(element =>
      element.hasAttribute(selector.slice(1, -1))),
    addEventListener: (name, callback) => {
      if (!listeners.has(name)) listeners.set(name, []);
      listeners.get(name).push(callback);
    },
    listenerCount: name => (listeners.get(name) || []).length
  };
}

test('bridge initialization is idempotent and normalizes legacy attributes', () => {
  const legacyToggle = fakeElement({'data-toggle': 'dropdown'});
  const legacyDismiss = fakeElement({'data-dismiss': 'modal'});
  const document = fakeDocument([legacyToggle, legacyDismiss]);
  bridge.init(document, null, null);
  bridge.init(document, null, null);
  assert.equal(document.listenerCount('click'), 1);
  assert.equal(document.listenerCount('DOMContentLoaded'), 1);
  assert.equal(legacyToggle.getAttribute('data-bs-toggle'), 'dropdown');
  assert.equal(legacyDismiss.getAttribute('data-bs-dismiss'), 'modal');
});

function jqueryFixture() {
  function Collection(element) { this.element = element; }
  Collection.prototype.each = function(callback) {
    callback.call(this.element);
    return this;
  };
  function $(element) { return new Collection(element); }
  $.fn = Collection.prototype;
  return $;
}

function componentFixture(calls) {
  return {
    getOrCreateInstance: () => ({
      show: () => { calls.show += 1; },
      hide: () => { calls.hide += 1; },
      toggle: () => { calls.toggle += 1; },
      dispose: () => { calls.dispose += 1; }
    })
  };
}

test('jQuery adapters dispatch Bootstrap component commands', () => {
  const calls = {show: 0, hide: 0, toggle: 0, dispose: 0};
  const $ = jqueryFixture();
  const bootstrap = {
    Modal: componentFixture(calls),
    Dropdown: componentFixture(calls),
    Popover: componentFixture(calls)
  };

  bridge.installJQueryAdapters($, bootstrap);
  $(fakeElement()).modal('hide');
  $(fakeElement()).dropdown('toggle');
  $(fakeElement()).popover('dispose');

  assert.equal(calls.hide, 1);
  assert.equal(calls.toggle, 1);
  assert.equal(calls.dispose, 1);
});
