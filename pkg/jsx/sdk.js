;(function () {
  'use strict'

  function Waterfall() {
    this._taps = []
  }
  Waterfall.prototype.tap = function (name, fn) {
    this._taps.push({ name: String(name || 'anonymous'), fn: fn })
  }
  Waterfall.prototype.runWaterfall = async function (input) {
    let value = input
    for (const tap of this._taps) {
      const out = await tap.fn(value)
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
    },
  }
})()
