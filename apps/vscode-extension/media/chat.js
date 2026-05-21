// Webview script for the Ironflyer chat panel.
//
// Owns: DOM rendering, composer state, streaming aggregation per turn.
// Talks to the extension host over `acquireVsCodeApi()`. Never speaks to
// the network directly — all backend calls go through the extension so
// the JWT never leaves SecretStorage.

(function () {
  // eslint-disable-next-line no-undef
  const vscode = acquireVsCodeApi();
  const log = document.getElementById('log');
  const form = document.getElementById('composer');
  const promptEl = document.getElementById('prompt');
  const roleEl = document.getElementById('role');
  const effortEl = document.getElementById('effort');
  const sendBtn = document.getElementById('send');
  const cancelBtn = document.getElementById('cancel');

  let current = null;

  form.addEventListener('submit', (e) => {
    e.preventDefault();
    const text = promptEl.value.trim();
    if (!text) return;
    appendTurn('user', text);
    promptEl.value = '';
    setBusy(true);
    vscode.postMessage({ type: 'prompt', text, role: roleEl.value, effort: effortEl.value });
  });

  cancelBtn.addEventListener('click', () => {
    vscode.postMessage({ type: 'cancel' });
    setBusy(false);
  });

  window.addEventListener('message', (event) => {
    const msg = event.data;
    switch (msg.type) {
      case 'turn-start':
        current = appendTurn('assistant', '');
        break;
      case 'sse':
        applySse(msg.event, msg.data);
        break;
      case 'turn-end':
        setBusy(false);
        current = null;
        break;
      case 'error':
        appendTurn('error', msg.message);
        setBusy(false);
        break;
      case 'lifecycle':
        appendLifecycle(msg.data);
        break;
    }
  });

  function appendLifecycle(data) {
    if (!data) return;
    const line = data.gate
      ? `${data.gate}: ${data.status || ''} ${data.message ? '— ' + data.message : ''}`
      : `${data.step || ''}: ${data.status || ''} ${data.message ? '— ' + data.message : ''}`;
    const turn = document.createElement('div');
    turn.className = 'turn lifecycle';
    const role = document.createElement('div');
    role.className = 'role';
    role.textContent = 'lifecycle';
    const body = document.createElement('div');
    body.className = 'body';
    body.textContent = line.trim();
    turn.appendChild(role);
    turn.appendChild(body);
    log.appendChild(turn);
    log.scrollTop = log.scrollHeight;
  }

  function applySse(eventName, data) {
    if (!current) current = appendTurn('assistant', '');
    switch (eventName) {
      case 'start':
        current.dataset.provider = data.provider || '';
        current.dataset.model = data.model || '';
        updateFooter(current);
        break;
      case 'text':
        appendText(current.querySelector('.body'), data.text || '');
        break;
      case 'thinking':
        showThinking(current, data.text || '');
        break;
      case 'cost':
        current.dataset.cost = data.totalUSD ?? data.cost ?? '';
        updateFooter(current);
        break;
      case 'done':
        updateFooter(current);
        break;
      case 'error':
        appendTurn('error', typeof data === 'string' ? data : JSON.stringify(data));
        break;
    }
    log.scrollTop = log.scrollHeight;
  }

  function appendTurn(kind, text) {
    const turn = document.createElement('div');
    turn.className = 'turn ' + kind;
    const role = document.createElement('div');
    role.className = 'role';
    role.textContent = kind;
    const body = document.createElement('div');
    body.className = 'body';
    body.textContent = text;
    const footer = document.createElement('div');
    footer.className = 'footer';
    turn.appendChild(role);
    turn.appendChild(body);
    turn.appendChild(footer);
    log.appendChild(turn);
    log.scrollTop = log.scrollHeight;
    return turn;
  }

  function appendText(bodyEl, text) {
    bodyEl.appendChild(document.createTextNode(text));
  }

  function showThinking(turn, text) {
    let t = turn.querySelector('.thinking');
    if (!t) {
      t = document.createElement('div');
      t.className = 'turn thinking';
      const r = document.createElement('div');
      r.className = 'role';
      r.textContent = 'thinking';
      const b = document.createElement('div');
      b.className = 'body';
      t.appendChild(r);
      t.appendChild(b);
      turn.appendChild(t);
    }
    t.querySelector('.body').appendChild(document.createTextNode(text));
  }

  function updateFooter(turn) {
    const f = turn.querySelector('.footer');
    const parts = [];
    if (turn.dataset.provider) parts.push(turn.dataset.provider + (turn.dataset.model ? '/' + turn.dataset.model : ''));
    if (turn.dataset.cost) parts.push('$' + Number(turn.dataset.cost).toFixed(5));
    f.textContent = parts.join(' · ');
  }

  function setBusy(b) {
    sendBtn.disabled = b;
    cancelBtn.hidden = !b;
    promptEl.disabled = b;
  }
})();
