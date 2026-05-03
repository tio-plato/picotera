;(function () {
  'use strict'

  function Waterfall() {
    this._taps = []
  }
  Waterfall.prototype.tap = function (name, fn, priority) {
    this._taps.push({ name: String(name || 'anonymous'), fn: fn, priority: priority ?? 0 })
    this._taps.sort(function (a, b) {
      return b.priority - a.priority;
    });
  }
  Waterfall.prototype.runWaterfall = async function (context, input) {
    let value = input
    for (const tap of this._taps) {
      const out = await tap.fn(context, value)
      if (typeof out !== 'undefined') {
        value = out
      }
    }
    return value
  }

  globalThis.picotera = {
    hooks: {
      sortProviders: new Waterfall(),
      beforeRequest: new Waterfall(),
      rewriteRequest: new Waterfall(),
      rewriteModel: new Waterfall(),
      rewriteProviderModels: new Waterfall(),
    },
    fetch: function (url, init) {
      var initJSON = init ? JSON.stringify(init) : ''
      return globalThis.__picotera_fetch(String(url), initJSON).then(function (s) {
        return JSON.parse(s)
      })
    },
  }

  var consoleEmit = function (level) {
    return function () {
      var parts = []
      for (var i = 0; i < arguments.length; i++) {
        var a = arguments[i]
        parts.push(typeof a === 'string' ? a : (function () { try { return JSON.stringify(a) } catch (_e) { return String(a) } })())
      }
      globalThis.__picotera_console(level, parts.join(' '))
    }
  }
  globalThis.console = {
    log: consoleEmit('info'),
    info: consoleEmit('info'),
    warn: consoleEmit('warn'),
    error: consoleEmit('error'),
    debug: consoleEmit('info'),
  }
})()
