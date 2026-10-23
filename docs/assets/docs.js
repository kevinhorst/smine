// The docs: the top nav (one menu per section, every page one click away), the page outline on the left,
// the design's terminal chrome on code windows, sortable tables.
(function () {
  var SECTIONS = [
    ['start', 'Start here', [
      ['install', 'Install'], ['configserver', 'Config server']
    ]],
    ['components', 'Components', [
      ['permissions', 'Permission System'], ['heartbeat', 'Heartbeat'], ['rust-binary', 'Agentwt'], ['peek', 'Peek'],
      ['smining', 'Smining'], ['routines', 'Routines'], ['settings', 'Settings'], ['trash', 'Trash']
    ]],
    ['context', 'Context', [
      ['context-system', 'Context system'], ['context-report', 'Context report'], ['acdsl', 'Context Linting']
    ]],
    ['skills', 'Skills', [
      ['skills', 'Skills directory'], ['skillcue', 'Skill model'], ['skill-map', 'Skill map'], ['skill-test-layers', 'Skill tests'],
      ['aliases', 'Aliases']
    ]],
    ['workarounds', 'Workarounds', [
      ['workflow-primer', 'Workflow primer']
    ]],
    ['research', 'Research', [
      ['cache-token', 'Caching'], ['token-usage', 'Token usage'], ['thinking', 'Thinking'], ['compaction', 'Compaction'],
      ['sota-acp', 'ACP'], ['sota-mcp', 'MCP'], ['sota-skills', 'Skills'], ['sota-telemetry', 'Telemetry']
    ]],
    ['development', 'Development', [
      ['development', 'Development']
    ]]
  ];
  var slug = decodeURIComponent(location.pathname.split('/').pop() || 'index.html').replace(/\.html$/, '') || 'index';
  var section = null;
  SECTIONS.forEach(function (entry) {
    entry[2].forEach(function (page) { if (page[0] === slug) { section = entry; } });
  });

  // Top nav: a menu per section; a one-page section is a plain link.
  var nav = document.createElement('nav');
  SECTIONS.forEach(function (entry) {
    var pages = entry[2];
    if (pages.length === 1) {
      var only = document.createElement('a');
      only.href = pages[0][0] + '.html';
      only.textContent = entry[1];
      if (section === entry) { only.className = 'active'; }
      nav.appendChild(only);
      return;
    }
    var menu = document.createElement('div');
    menu.className = 'nav-menu';
    menu.tabIndex = 0;
    var head = document.createElement('a');
    head.className = 'nav-section' + (section === entry ? ' active' : '');
    head.href = pages[0][0] + '.html';
    head.textContent = entry[1];
    menu.appendChild(head);
    var drop = document.createElement('div');
    drop.className = 'nav-drop';
    pages.forEach(function (page) {
      var link = document.createElement('a');
      link.href = page[0] + '.html';
      link.textContent = page[1];
      if (page[0] === slug) { link.className = 'active'; }
      drop.appendChild(link);
    });
    menu.appendChild(drop);
    nav.appendChild(menu);
  });
  var brand = document.createElement('a');
  brand.className = 'nav-brand';
  brand.href = 'index.html';
  brand.textContent = 'smine.';
  var version = document.createElement('span');
  version.className = 'nav-version';
  version.textContent = 'docs';
  brand.appendChild(version);
  nav.appendChild(brand);

  // Layout: nav on top, the outline on the left, the page in main.
  var main = document.createElement('main');
  main.className = 'doc-main';
  while (document.body.firstChild) { main.appendChild(document.body.firstChild); }
  var aside = document.createElement('aside');
  aside.className = 'outline';
  var title = document.createElement('div');
  title.className = 'outline-title';
  title.textContent = 'On this page';
  aside.appendChild(title);
  var headings = main.querySelectorAll('h2, h3');
  var links = [];
  headings.forEach(function (heading) {
    if (!heading.id) {
      heading.id = heading.textContent.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
    }
    var link = document.createElement('a');
    link.href = '#' + heading.id;
    link.textContent = heading.textContent.trim();
    link.className = heading.tagName.toLowerCase();
    aside.appendChild(link);
    links.push({ heading: heading, link: link });
  });
  document.body.appendChild(nav);
  if (links.length > 1) { document.body.appendChild(aside); } else { document.body.classList.add('no-outline'); }
  document.body.appendChild(main);

  // Scroll spy: the last heading above the fold is the current one. At the page bottom the last headings can
  // never reach the fold, so the last one wins there, and a clicked entry stays marked until the reader scrolls.
  var clicked = null;
  function mark(current) {
    links.forEach(function (item) { item.link.classList.toggle('active', item === current); });
  }
  function spy() {
    if (clicked) { mark(clicked); return; }
    var current = null;
    links.forEach(function (item) {
      if (item.heading.getBoundingClientRect().top <= 96) { current = item; }
    });
    var atBottom = window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 2;
    if (atBottom) { current = links[links.length - 1]; }
    mark(current);
  }
  function release() { clicked = null; }
  if (links.length > 1) {
    links.forEach(function (item) {
      item.link.addEventListener('click', function () { clicked = item; mark(item); });
    });
    ['wheel', 'touchstart', 'keydown'].forEach(function (name) {
      window.addEventListener(name, release, { passive: true });
    });
    window.addEventListener('scroll', spy, { passive: true });
    spy();
  }
})();

(function () {
  var DOTS = [{ left: 30, title: 'minimize' }, { left: 48, title: 'expand' }];
  document.querySelectorAll('pre.file-view').forEach(function (pre) {
    DOTS.forEach(function (spec) {
      var dot = document.createElement('button');
      dot.type = 'button';
      dot.className = 'term-dot';
      dot.title = spec.title;
      dot.style.left = spec.left + 'px';
      dot.addEventListener('click', function () {
        if (spec.title === 'minimize') {
          pre.classList.remove('maximized');
          pre.classList.toggle('minimized');
          return;
        }
        pre.classList.remove('minimized');
        pre.classList.toggle('maximized');
      });
      pre.prepend(dot);
    });
  });
  document.addEventListener('keydown', function (evt) {
    if (evt.key !== 'Escape') { return; }
    document.querySelectorAll('pre.file-view.maximized').forEach(function (pre) { pre.classList.remove('maximized'); });
  });
})();

(function () {
  function cellText(row, column) {
    var cell = row.children[column];
    return cell ? cell.textContent.trim() : '';
  }
  function compare(a, b) {
    var na = parseFloat(a.replace(/,/g, ''));
    var nb = parseFloat(b.replace(/,/g, ''));
    if (!isNaN(na) && !isNaN(nb)) { return na - nb; }
    return a.toLowerCase().localeCompare(b.toLowerCase());
  }
  document.querySelectorAll('table[data-sortable] thead th').forEach(function (th) {
    th.addEventListener('click', function () {
      var table = th.closest('table');
      var body = table.tBodies[0];
      if (!body) { return; }
      var dir = th.getAttribute('data-dir') === 'asc' ? 'desc' : 'asc';
      table.querySelectorAll('thead th').forEach(function (other) { other.removeAttribute('data-dir'); });
      th.setAttribute('data-dir', dir);
      var column = th.cellIndex;
      var rows = Array.prototype.slice.call(body.rows);
      rows.sort(function (a, b) {
        var order = compare(cellText(a, column), cellText(b, column));
        return dir === 'asc' ? order : -order;
      });
      rows.forEach(function (row) { body.appendChild(row); });
    });
  });
})();
