// Wails v2 injects window.go.main.App.* into WebView2 at startup.
// Using this global avoids any dependency on the auto-generated wailsjs/ files.

(function () {
  'use strict';

  // ── State ──────────────────────────────────────────────────────────────────

  let currentOp = 'set';

  // ── DOM refs ───────────────────────────────────────────────────────────────

  const hostInput       = document.getElementById('host');
  const portInput       = document.getElementById('port');
  const btnConnect      = document.getElementById('btn-connect');
  const btnDisconnect   = document.getElementById('btn-disconnect');
  const statusDot       = document.getElementById('status-dot');
  const statusText      = document.getElementById('status-text');
  const keyInput        = document.getElementById('key-input');
  const valueInput      = document.getElementById('value-input');
  const ttlInput        = document.getElementById('ttl-input');
  const valueRow        = document.getElementById('value-row');
  const ttlRow          = document.getElementById('ttl-row');
  const outputLog       = document.getElementById('output-log');
  const btnExecute      = document.getElementById('btn-execute');
  const btnClear        = document.getElementById('btn-clear');
  const btnRefreshStats = document.getElementById('btn-refresh-stats');
  const btnSnapshot     = document.getElementById('btn-snapshot');

  // Shorthand to the bound Go methods
  function api() {
    return window.go.main.App;
  }

  // ── Tabs ───────────────────────────────────────────────────────────────────

  document.querySelectorAll('.tab').forEach(function (tab) {
    tab.addEventListener('click', function () {
      document.querySelectorAll('.tab').forEach(function (t) {
        t.classList.remove('tab--active');
      });
      tab.classList.add('tab--active');
      currentOp = tab.dataset.op;
      updateForm();
    });
  });

  function updateForm() {
    var isSet = currentOp === 'set';
    valueRow.style.display = isSet ? '' : 'none';
    ttlRow.style.display   = isSet ? '' : 'none';
  }

  updateForm();

  // ── Connection ─────────────────────────────────────────────────────────────

  btnConnect.addEventListener('click', function () {
    var host = hostInput.value.trim() || 'localhost';
    var port = parseInt(portInput.value, 10) || 6380;
    appendLog('> CONNECT ' + host + ':' + port);
    api().Connect(host, port).then(function (res) {
      if (res.success) {
        setConnected(true);
        appendLog('  ' + res.message, 'ok');
      } else {
        appendLog('  Error: ' + res.message, 'err');
      }
    }).catch(function (e) {
      appendLog('  Error: ' + e, 'err');
    });
  });

  btnDisconnect.addEventListener('click', function () {
    api().Disconnect().then(function (res) {
      setConnected(false);
      appendLog('> DISCONNECT');
      appendLog('  ' + res.message, 'ok');
    }).catch(function (e) {
      appendLog('  Error: ' + e, 'err');
    });
  });

  function setConnected(connected) {
    statusDot.className    = 'status-indicator' + (connected ? ' status-indicator--on' : '');
    statusText.textContent = connected ? 'Connected' : 'Disconnected';
    btnConnect.disabled    = connected;
    btnDisconnect.disabled = !connected;
  }

  // ── Execute ────────────────────────────────────────────────────────────────

  btnExecute.addEventListener('click', function () {
    var key = keyInput.value.trim();
    if (!key) {
      appendLog('  Error: key is required', 'err');
      return;
    }

    var p;
    switch (currentOp) {
      case 'set': {
        var value  = valueInput.value;
        var ttl    = parseInt(ttlInput.value, 10) || 0;
        var suffix = ttl > 0 ? ' ttl ' + ttl : '';
        appendLog('> SET ' + key + ' "' + value + '"' + suffix);
        p = api().SetKey(key, value, ttl);
        break;
      }
      case 'get':
        appendLog('> GET ' + key);
        p = api().GetKey(key);
        break;
      case 'del':
        appendLog('> DEL ' + key);
        p = api().DeleteKey(key);
        break;
      case 'exists':
        appendLog('> EXISTS ' + key);
        p = api().ExistsKey(key);
        break;
      case 'ttl':
        appendLog('> TTL ' + key);
        p = api().GetTTL(key);
        break;
      default:
        return;
    }

    p.then(function (res) {
      appendLog('  ' + res.message, res.success ? 'ok' : 'err');
    }).catch(function (e) {
      appendLog('  Error: ' + e, 'err');
    });
  });

  // Enter key in any input triggers Execute
  [keyInput, valueInput, ttlInput].forEach(function (el) {
    el.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') { btnExecute.click(); }
    });
  });

  // ── Output log ─────────────────────────────────────────────────────────────

  btnClear.addEventListener('click', function () {
    outputLog.innerHTML = '';
  });

  function appendLog(text, cls) {
    var line = document.createElement('div');
    line.className = 'log-line' + (cls ? ' log-line--' + cls : '');
    line.textContent = text;
    outputLog.appendChild(line);
    outputLog.scrollTop = outputLog.scrollHeight;
  }

  // ── Stats ──────────────────────────────────────────────────────────────────

  btnRefreshStats.addEventListener('click', function () {
    api().GetStats().then(function (res) {
      if (res.success && res.data) {
        renderStats(res.data);
      } else {
        appendLog('> STATS');
        appendLog('  Error: ' + res.message, 'err');
      }
    }).catch(function (e) {
      appendLog('  Error: ' + e, 'err');
    });
  });

  function renderStats(data) {
    document.getElementById('stat-keys').textContent   = data.keys;
    document.getElementById('stat-memory').textContent = formatBytes(data.memory);
    document.getElementById('stat-hits').textContent   = data.hits;
    document.getElementById('stat-misses').textContent = data.misses;
    document.getElementById('stat-uptime').textContent = formatDuration(data.uptime);

    var reqs  = data.requests || {};
    var parts = Object.keys(reqs).sort().map(function (k) {
      return k + '=' + reqs[k];
    }).join('  ');
    document.getElementById('stats-requests').textContent =
      parts ? 'Requests:  ' + parts : '';
  }

  function formatBytes(bytes) {
    if (bytes < 1024)        return bytes + ' B';
    if (bytes < 1048576)     return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / 1048576).toFixed(1) + ' MB';
  }

  function formatDuration(seconds) {
    if (seconds < 60) return seconds + 's';
    var h = Math.floor(seconds / 3600);
    var m = Math.floor((seconds % 3600) / 60);
    var s = seconds % 60;
    if (h > 0) return h + 'h ' + m + 'm';
    return m + 'm ' + s + 's';
  }

  // ── Snapshot ───────────────────────────────────────────────────────────────

  btnSnapshot.addEventListener('click', function () {
    appendLog('> SNAPSHOT');
    api().TriggerSnapshot().then(function (res) {
      appendLog('  ' + res.message, res.success ? 'ok' : 'err');
    }).catch(function (e) {
      appendLog('  Error: ' + e, 'err');
    });
  });

}());
