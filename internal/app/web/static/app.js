(function () {
  'use strict';

  function byId(id) { return document.getElementById(id); }

  function money(v) {
    try {
      return new Intl.NumberFormat('vi-VN').format(v) + ' VNĐ';
    } catch (e) {
      return String(v) + ' VNĐ';
    }
  }

  function digits(value) {
    var s = String(value || '').replace(/[^\d]/g, '');
    return s ? parseInt(s, 10) : 0;
  }

  function updateNetPreview() {
    var gross = byId('gross_value');
    var invest = byId('invest_cost');
    var out = byId('net-preview');
    if (!gross || !invest || !out) return;
    var net = Math.max(0, digits(gross.value) - digits(invest.value));
    out.textContent = money(net);
  }

  function syncTrackCards() {
    var inputs = document.querySelectorAll('input[name="track"]');
    for (var i = 0; i < inputs.length; i++) {
      var input = inputs[i];
      var card = input.closest('.radio-card');
      if (card) card.classList.toggle('selected', input.checked);
    }
  }

  document.addEventListener('click', function (e) {
    var t = e.target;
    if (t.closest('[data-toggle-sidebar]')) {
      document.body.classList.toggle('sidebar-open');
      return;
    }
    if (t.closest('[data-close-sidebar]')) {
      document.body.classList.remove('sidebar-open');
      return;
    }
    if (t.closest('[data-print]')) {
      window.print();
    }
  });

  document.addEventListener('change', function (e) {
    if (e.target && e.target.name === 'track') syncTrackCards();
  });

  document.addEventListener('input', function (e) {
    if (!e.target) return;
    if (e.target.id === 'gross_value' || e.target.id === 'invest_cost') updateNetPreview();
  });

  function init() {
    syncTrackCards();
    updateNetPreview();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
