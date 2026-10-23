// Backlog board panel: an item chip opens the item's card in the panel floating over the
// top-right corner, built from the #backlog-data JSON the page embeds. The panel bar is the
// card's header — item page icon, status and priority selects, badges, close — and the body
// holds the text, evidence and dispatch form; htmx.process wires the inserted controls.
(() => {
  'use strict';
  const dataNode = document.getElementById('backlog-data');
  if (!dataNode) { return; }
  const data = JSON.parse(dataNode.textContent);
  const panel = document.getElementById('bl-panel');
  const head = document.getElementById('bl-head');
  const body = document.getElementById('bl-panel-body');
  const priLabels = { critical: 'crit', medium: 'med' };
  const iconItemPage = '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">'
    + '<path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/></svg>';
  const escapeHtml = (text) => String(text)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  const badge = (text, cls) => `<span class="badge ${cls}">${escapeHtml(text)}</span>`;
  const select = (name, route, values, current, cls, label) => `<select name="${name}" class="${cls}" hx-post="${route}" hx-swap="none">`
    + values.map((value) => `<option value="${escapeHtml(value)}"${value === current ? ' selected' : ''}>${escapeHtml(label(value))}</option>`).join('') + `</select>`;
  const sessionLink = (entry) => {
    if (entry.weekUrl) { return `<a href="${escapeHtml(entry.weekUrl)}" title="open the session findings">${escapeHtml(entry.text)}</a>`; }
    return `<span>${escapeHtml(entry.text)}</span>`;
  };
  const depItem = (dep) => dep.active
    ? `<a class="dep-link" href="#item-${escapeHtml(dep.id)}" title="depends on ${escapeHtml(dep.id)}">needs ${escapeHtml(dep.id)}</a>`
    : `<span class="meta" title="depends on ${escapeHtml(dep.id)}, which is archived">needs ${escapeHtml(dep.id)} · archived</span>`;
  const section = (label, inner) => `<div class="bl-section"><span class="section-label">${label}</span>${inner}</div>`;
  // dispatchState: the chosen routine's state is the button's tooltip and decides whether it is enabled.
  const dispatchState = (item, routine) => {
    if (!routine) {
      return { isEnabled: false, tip: `no backlog routine runs /${item.routeSkill} — create one with /skillroutine-create routine` };
    }
    const state = routine.loaded ? 'loaded' : 'stopped — Start it on the Routines page';
    const live = routine.live ? ' · a run is active' : '';
    const stage = item.stage ? ` · records status ${item.stage}` : '';
    return { isEnabled: routine.loaded && !routine.live, tip: `${routine.name} · ${state}${live}${stage}` };
  };
  // dispatchHtml: the routine select sits above the prompt; a disabled button sits in a span so the tooltip still shows.
  const dispatchHtml = (item) => {
    if (!item.routeSkill) { return ''; }
    const options = item.routines || [];
    const chosen = options.find((routine) => routine.name === item.routine) || options[0];
    const { isEnabled, tip } = dispatchState(item, chosen);
    const routineSelect = options.length
      ? `<select name="routine" autocomplete="off" title="the backlog routine this dispatch runs on; its env sets model, effort and budget">`
        + options.map((routine) => `<option value="${escapeHtml(routine.name)}"${routine === chosen ? ' selected' : ''}>${escapeHtml(routine.name)}</option>`).join('')
        + `</select>`
      : '';
    // board=1: the answer swaps the item's Activity cell, which shows "starting", then "running", until the run ends; the sent form keeps its button disabled.
    return `<form class="bl-dispatch" hx-post="/api/backlog/${escapeHtml(item.id)}/dispatch" hx-target="#op-result" hx-disabled-elt="find button"`
      + ` hx-on::after-request="if (event.detail.successful) { this.querySelector('button').disabled = true; }"`
      + ` hx-confirm="Dispatch ${escapeHtml(item.id)} now? Budget and model come from the chosen routine's env.">`
      + routineSelect
      + `<input type="hidden" name="board" value="1">`
      + `<input type="text" name="prompt" placeholder="extra prompt for this run">`
      + `<span class="bl-dispatch-tip" title="${escapeHtml(tip)}"><button type="submit"${isEnabled ? '' : ' disabled'}>Dispatch</button></span>`
      + `</form>`;
  };
  // A routine choice re-reads that routine's state into the button.
  body.addEventListener('change', (event) => {
    const routineSelect = event.target.closest('.bl-dispatch select[name="routine"]');
    if (!routineSelect) { return; }
    const item = data.items[panel.dataset.itemId];
    const chosen = (item.routines || []).find((routine) => routine.name === routineSelect.value);
    const { isEnabled, tip } = dispatchState(item, chosen);
    const form = routineSelect.closest('form');
    form.querySelector('.bl-dispatch-tip').title = tip;
    form.querySelector('button').disabled = !isEnabled;
  });
  const buildHead = (item) => `<a class="icon-button" href="/backlog/${escapeHtml(item.id)}" title="open the full item page" aria-label="open the full item page">${iconItemPage}</a>`
    + select('status', `/api/backlog/${escapeHtml(item.id)}/status`, data.statuses, item.status, '', (value) => value)
    + select('pri', `/api/backlog/${escapeHtml(item.id)}/pri`, data.priorities, item.pri, `badge badge-select ${item.priClass}`, (value) => priLabels[value] || value)
    + badge(item.size, 'badge-dim') + badge(item.theme, 'badge-dim')
    + `${item.mined ? `<span class="badge badge-ok" title="smined ${escapeHtml(item.mined.date)} by the smine nightly">smined</span>` : ''}`;
  // minedBy names the run session that mined the row, linked to peek; rows mined before it was recorded show the date alone.
  const minedBy = (item) => {
    const mined = item.mined;
    if (!mined.by) { return `smined ${escapeHtml(mined.date)}`; }
    const short = escapeHtml(mined.by.slice(0, 8));
    const session = item.minedByUrl
      ? `<a href="${escapeHtml(item.minedByUrl)}" title="${escapeHtml(mined.by)} — the run that mined this row, in peek">${short}</a>`
      : `<span title="${escapeHtml(mined.by)}">${short}</span>`;
    return `smined ${escapeHtml(mined.date)} by ${session}`;
  };
  const buildBody = (item) => {
    const deps = item.deps || [];
    const evidence = item.evidence || [];
    let html = `<div class="bl-ident"><span class="bl-chip">${escapeHtml(item.id)}</span><span class="bl-ident-title">${escapeHtml(item.title)}</span></div>`;
    html += `<hr class="bl-rule">`;
    html += `<div class="item-desc">${escapeHtml(item.description)}</div>`;
    if (item.doneWhen) { html += section('acceptance', `<div class="item-desc">${escapeHtml(item.doneWhen)}</div>`); }
    if (evidence.length) { html += section('evidence', `<ul class="item-evidence bl-evidence">${evidence.map((entry) => `<li>${sessionLink(entry)}</li>`).join('')}</ul>`); }
    if (item.mined) { html += section('from', `<div class="bl-origin">${minedBy(item)}</div>`); }
    if (deps.length) { html += section('deps', `<div>${deps.map(depItem).join(' ')}</div>`); }
    html += dispatchHtml(item);
    return html;
  };
  const close = () => {
    head.innerHTML = '';
    body.innerHTML = '';
    panel.hidden = true;
    delete panel.dataset.itemId;
    document.querySelectorAll('.bl-chip.bl-selected').forEach((chip) => chip.classList.remove('bl-selected'));
  };
  const openCard = (id) => {
    const item = data.items[id];
    if (!item) { return; }
    head.innerHTML = buildHead(item);
    body.innerHTML = buildBody(item);
    panel.dataset.itemId = id;
    panel.hidden = false;
    markSelected();
    if (window.htmx) { window.htmx.process(panel); }
  };
  const markSelected = () => {
    document.querySelectorAll('.bl-chip[data-item-chip]').forEach((chip) => {
      chip.classList.toggle('bl-selected', chip.dataset.itemChip === panel.dataset.itemId);
    });
  };
  // backlog:item carries an item's fresh card data after an in-place write: the row's filter attributes follow, the card
  // header re-renders, its body (and anything typed into the prompt) stays unless the route changed, the page never reloads.
  document.body.addEventListener('backlog:item', (event) => {
    const fresh = Object.assign({}, event.detail);
    delete fresh.elt;
    const previous = data.items[fresh.id];
    data.items[fresh.id] = fresh;
    const row = document.getElementById(`item-${fresh.id}`);
    if (row) {
      row.dataset.status = fresh.status;
      row.dataset.pri = fresh.pri;
    }
    if (panel.dataset.itemId !== fresh.id) { return; }
    head.innerHTML = buildHead(fresh);
    if (window.htmx) { window.htmx.process(head); }
    const isRouteChanged = !previous || previous.routeSkill !== fresh.routeSkill;
    if (!isRouteChanged) { return; }
    const prompt = body.querySelector('input[name="prompt"]');
    const typed = prompt ? prompt.value : '';
    body.innerHTML = buildBody(fresh);
    const rebuilt = body.querySelector('input[name="prompt"]');
    if (rebuilt) { rebuilt.value = typed; }
    if (window.htmx) { window.htmx.process(body); }
  });
  document.addEventListener('click', (event) => {
    const chip = event.target.closest('.bl-chip[data-item-chip]');
    if (!chip) { return; }
    if (panel.dataset.itemId === chip.dataset.itemChip) { close(); return; }
    openCard(chip.dataset.itemChip);
  });
  document.getElementById('bl-close').addEventListener('click', close);
})();
