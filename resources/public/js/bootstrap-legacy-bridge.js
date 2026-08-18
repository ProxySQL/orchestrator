(function(root) {
  var clickListenerInstalled = false;
  var readyListenerInstalled = false;
  var bootstrapAPI = null;

  function normalizeAttributes(scope) {
    var mappings = [
      ['data-toggle', 'data-bs-toggle'],
      ['data-target', 'data-bs-target'],
      ['data-dismiss', 'data-bs-dismiss']
    ];
    mappings.forEach(function(mapping) {
      scope.querySelectorAll('[' + mapping[0] + ']').forEach(function(element) {
        if (!element.hasAttribute(mapping[1])) {
          element.setAttribute(mapping[1], element.getAttribute(mapping[0]));
        }
      });
    });
  }

  function installJQueryAdapters($, bootstrap) {
    if (!$ || !$.fn || !bootstrap) return;
    var commandAliases = {destroy: 'dispose'};
    function install(name, Component) {
      if (!Component) return;
      $.fn[name] = function(commandOrOptions) {
        return this.each(function() {
          var options = typeof commandOrOptions === 'object' ? commandOrOptions : undefined;
          var instance = Component.getOrCreateInstance(this, options);
          if (typeof commandOrOptions === 'string') {
            var command = commandAliases[commandOrOptions] || commandOrOptions;
            if (typeof instance[command] === 'function') instance[command]();
          } else if (name === 'modal' && (!options || options.show !== false)) {
            instance.show();
          }
        });
      };
    }
    install('modal', bootstrap.Modal);
    install('dropdown', bootstrap.Dropdown);
    install('popover', bootstrap.Popover);
    install('tooltip', bootstrap.Tooltip);
    install('alert', bootstrap.Alert);
  }

  function handleLegacyDismissals(event) {
    var trigger = event.target.closest('[data-dismiss]');
    if (!trigger || !bootstrapAPI) return;
    var kind = trigger.getAttribute('data-dismiss');
    var host = trigger.closest(kind === 'modal' ? '.modal' : '.alert');
    if (!host) return;
    if (kind === 'modal') bootstrapAPI.Modal.getOrCreateInstance(host).hide();
    if (kind === 'alert') bootstrapAPI.Alert.getOrCreateInstance(host).close();
  }

  function init(doc, $, bootstrap) {
    bootstrapAPI = bootstrap || bootstrapAPI;
    normalizeAttributes(doc);
    installJQueryAdapters($, bootstrap);
    if (!clickListenerInstalled) {
      doc.addEventListener('click', handleLegacyDismissals);
      clickListenerInstalled = true;
    }
    if (!readyListenerInstalled) {
      doc.addEventListener('DOMContentLoaded', function() {
        normalizeAttributes(doc);
        installJQueryAdapters($, bootstrap);
      });
      readyListenerInstalled = true;
    }
  }

  var api = {
    init: init,
    normalizeAttributes: normalizeAttributes,
    installJQueryAdapters: installJQueryAdapters
  };
  root.OrchestratorBootstrapBridge = api;
  if (typeof module === 'object' && module.exports) module.exports = api;
})(typeof window === 'undefined' ? globalThis : window);
