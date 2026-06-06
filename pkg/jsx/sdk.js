;(function () {
  'use strict'

  function Waterfall() {
    this._taps = []
  }
  Waterfall.prototype.tap = function (name, fn, priority) {
    this._taps.push({ name: String(name || 'anonymous'), fn: fn, priority: priority ?? 0 })
    this._taps.sort(function (a, b) {
      return a.priority - b.priority;
    });
  }
  Waterfall.prototype.runWaterfall = function (context, input) {
    let value = input
    for (const tap of this._taps) {
      const out = tap.fn(context, value)
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
      beforeTransform: new Waterfall(),
      rewriteRequest: new Waterfall(),
      rewriteModel: new Waterfall(),
      rewriteProviderModels: new Waterfall(),
    },
    kv: {
      get: function (key) {
        var r = globalThis.__picotera_kv_get(String(key));
        if (r[1]) throw new Error(r[1]);
        return r[0] === '' ? null : JSON.parse(r[0]);
      },
      set: function (key, value) {
        var e = globalThis.__picotera_kv_set(String(key), JSON.stringify(value));
        if (e) throw new Error(e);
      },
      setex: function (key, seconds, value) {
        var e = globalThis.__picotera_kv_setex(String(key), Number(seconds), JSON.stringify(value));
        if (e) throw new Error(e);
      },
      ttl: function (key) {
        var r = globalThis.__picotera_kv_ttl(String(key));
        if (r[1]) throw new Error(r[1]);
        return r[0];
      },
      del: function (key) {
        var e = globalThis.__picotera_kv_del(String(key));
        if (e) throw new Error(e);
      },
    },
    fetch: function (url, init) {
      var initJSON = init ? JSON.stringify(init) : ''
      var r = globalThis.__picotera_fetch(String(url), initJSON)
      if (r[1]) throw new Error(r[1])
      return JSON.parse(r[0])
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
