'use strict';

const $ = (id) => document.getElementById(id);

// The language lang.js chose, German or English: T(de, en) is the text in it. The page's own
// texts are there in both, the other hidden; what is made here goes through T.
const LANG = document.documentElement.dataset.lang === 'en' ? 'en' : 'de';
const T = (de, en) => (LANG === 'en' ? en : de);
const LOCALE = T('de-DE', 'en-GB');

// The page's attributes in English: data-en-title, data-en-placeholder, data-en-aria-label.
if (LANG === 'en') {
  for (const name of ['title', 'placeholder', 'aria-label']) {
    for (const node of document.querySelectorAll(`[data-en-${name}]`)) node.setAttribute(name, node.getAttribute(`data-en-${name}`));
  }
}

/** Makes an element: el('li', 'row', child, 'text', …); falsy children are left out. */
function el(tag, className, ...children) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  for (const child of children) {
    if (child !== null && child !== undefined && child !== false && child !== '') node.append(child);
  }
  return node;
}

function button(text, className, onClick) {
  const b = el('button', className, text);
  b.type = 'button';
  b.addEventListener('click', () => onClick(b));
  return b;
}

async function api(method, path, body) {
  const isForm = body instanceof FormData;
  const res = await fetch(path, {
    method,
    // The server answers in the page's language: its messages, that is.
    headers: { 'X-Lang': LANG, ...(body && !isForm ? { 'Content-Type': 'application/json' } : {}) },
    body: isForm ? body : body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  // The session gone — expired, or ended elsewhere: back to the login.
  if (res.status === 401) location.href = '/login';
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
}

// Abmelden, once logged in: the session ends here, the login comes again.
(async () => {
  const session = await api('GET', '/api/session').catch(() => null);
  if (!session?.user) return;
  $('logout').hidden = false;
  $('logout').title = T(`Angemeldet als ${session.user}`, `Logged in as ${session.user}`);
  $('logout').addEventListener('click', async () => {
    await api('POST', '/api/logout').catch(() => {});
    location.href = '/login';
  });
})();

const euro = (v) => `${(v ?? 0).toFixed(2).replace('.', ',')} €`;
const dmy = (iso) => (iso ? iso.split('-').reverse().join('.') : '');
const MONTHS = T(['Januar', 'Februar', 'März', 'April', 'Mai', 'Juni', 'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember'],
  ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']);
const never = (iso) => !iso || iso.startsWith('0001');

function ago(iso) {
  if (never(iso)) return '';
  const s = Math.round((Date.now() - new Date(iso)) / 1000);
  if (s < 60) return T('gerade eben', 'just now');
  if (s < 3600) return T(`vor ${Math.round(s / 60)} min`, `${Math.round(s / 60)} min ago`);
  if (s < 86400) return T(`vor ${Math.round(s / 3600)} h`, `${Math.round(s / 3600)} h ago`);
  return new Date(iso).toLocaleString(LOCALE, { dateStyle: 'short', timeStyle: 'short' });
}

// ── Theme and language, as on rummeyer.com ─────────────────────────────────

// Three states in a ring, the first click always away from what is on screen: on a light system
// auto → dark → light → auto, on a dark one auto → light → dark → auto.
function nextTheme(pref) {
  const systemDark = window.matchMedia?.('(prefers-color-scheme: dark)').matches;
  const away = systemDark ? 'light' : 'dark';
  if (pref === 'auto') return away;
  if (pref === away) return systemDark ? 'dark' : 'light';
  return 'auto';
}

function labelTheme() {
  const pref = document.documentElement.dataset.themePref || 'auto';
  const name = T({ light: 'Hell-Modus', dark: 'Dunkel-Modus', auto: 'System-Modus' }, { light: 'light mode', dark: 'dark mode', auto: 'system mode' })[pref];
  $('theme').title = name;
  $('theme').setAttribute('aria-label', T(`Zwischen hell, dunkel und System wechseln (aktuell ${name})`, `Switch between light, dark and system mode (currently ${name})`));
}

$('theme').addEventListener('click', () => {
  const root = document.documentElement;
  const next = nextTheme(root.dataset.themePref || 'auto');
  root.dataset.themePref = next;
  // Auto is the key's absence: following the system, not a snapshot of what it said that day.
  try {
    if (next === 'auto') localStorage.removeItem('theme');
    else localStorage.setItem('theme', next);
  } catch { /* private window */ }
  if (next === 'auto') delete root.dataset.theme;
  else root.dataset.theme = next;
  labelTheme();
});
labelTheme();

// The language: the page loads anew in it — what it made itself is in the language it was made
// in. Changes not saved yet: the browser asks first, as on leaving.
for (const link of document.querySelectorAll('[data-lang-link]')) {
  link.setAttribute('aria-current', String(link.dataset.langLink === LANG));
  link.addEventListener('click', (e) => {
    e.preventDefault();
    if (link.dataset.langLink === LANG) return;
    location.href = `?lang=${link.dataset.langLink}${location.hash}`;
  });
}

// ── Tabs ───────────────────────────────────────────────────────────────────

const TABS = ['cron', 'travel', 'receipts', 'invoice', 'log', 'settings'];
const loaders = {};

function showTab() {
  const name = TABS.includes(location.hash.slice(1)) ? location.hash.slice(1) : 'cron';
  for (const t of TABS) $(`tab-${t}`).hidden = t !== name;
  // The Protokoll has nothing to save: its button empties it instead.
  for (const id of ['save-all', 'save-note', 'discard-all']) $(id).hidden = name === 'log';
  for (const id of ['clear-log', 'clear-note']) $(id).hidden = name !== 'log';
  for (const a of document.querySelectorAll('.tabs a')) a.classList.toggle('on', a.getAttribute('href') === `#${name}`);
  loaders[name]?.();
}
window.addEventListener('hashchange', showTab);

// ── Overview and status ────────────────────────────────────────────────────

let overview = null;

async function loadOverview() {
  try {
    overview = await api('GET', '/api/overview');
  } catch (err) {
    $('status').textContent = T('nicht erreichbar', 'not reachable');
    return;
  }
  $('version').textContent = overview.version;
  const status = $('status');
  status.classList.toggle('busy', !!overview.running);
  status.textContent = overview.running ? title(overview.running.title) : nextText(overview.next);
  $('userbar').hidden = !overview.employee;
  $('employee').textContent = `${T('Mitarbeiter', 'Employee')}: ${overview.employee}`;
  for (const p of document.querySelectorAll('[data-last]')) p.textContent = lastText(overview.last[p.dataset.last]);
  for (const span of document.querySelectorAll('[data-mail-to]')) span.textContent = mailTo();
}

const SHORT = { 'travel-expense': T('Reisekosten', 'Travel'), 'anthropic-invoice': 'Anthropic', 'apple-invoice': 'Apple', 'vodafone-invoice': 'Vodafone', 'harvest-invoice': T('Kundenrechnung', 'Invoice') };

/** "nächster Lauf: Vodafone · Sa., 21.11. 07:30", from the schedule. */
function nextText(next) {
  if (!next) return T('kein Lauf geplant', 'no run planned');
  const at = new Date(next.at).toLocaleString(LOCALE, { weekday: 'short', day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
  return `${T('nächster Lauf', 'next run')}: ${SHORT[next.job] || next.job} · ${at}`;
}

/** Where the mails go: Einstellungen › E-Mail, its recipient and any copy. */
function mailTo() {
  const m = overview?.mail;
  if (!m?.to) return T('den Empfänger aus Einstellungen › E-Mail (noch keiner eingetragen)', 'the recipient from Settings › E-mail (none entered yet)');
  return m.cc ? `${m.to} (${T('Kopie', 'copy')} ${m.cc})` : m.to;
}

/** A run's state in words. */
const STATUS = () => T({ running: 'läuft', ok: 'erledigt', failed: 'fehlgeschlagen' }, { running: 'running', ok: 'done', failed: 'failed' });

function lastText(job) {
  if (!job) return '';
  const how = STATUS()[job.status] || job.status;
  return `${title(job.title)} — ${how}, ${ago(job.ended || job.started)}${job.error ? `: ${job.error}` : ''}`;
}

// The server names runs in German, kept so in the log: in English, the names it gives.
const TITLES = [
  [/^Reisekosten senden \(laufender Monat\)/, 'Send travel expenses (current month)'],
  [/^Anthropic-Belege abholen/, 'Fetch Anthropic receipts'],
  [/^Apple-Rechnungen abholen/, 'Fetch Apple invoices'],
  [/^Vodafone-Rechnungen abholen/, 'Fetch Vodafone invoices'],
  [/^Kundenrechnung aus Harvest \(laufender Monat\)/, 'Invoice from Harvest (current month)'],
  [/^Reisekosten (\S+) erzeugen/, 'Travel expenses $1, view'],
  [/^Reisekosten (\S+) senden/, 'Travel expenses $1, send'],
  [/^Anthropic-Belege (\S+) ansehen/, 'Anthropic receipts $1, view'],
  [/^Anthropic-Belege (\S+) senden/, 'Anthropic receipts $1, send'],
  [/^(Apple|Vodafone)-Rechnungen (\S+) ansehen/, '$1 invoices $2, view'],
  [/^(Apple|Vodafone)-Rechnungen (\S+) senden/, '$1 invoices $2, send'],
  [/^Kundenrechnung (\S+) aus Harvest/, 'Invoice $1 from Harvest'],
  [/^Zeitnachweis (\S+) aus Harvest/, 'Work log $1 from Harvest'],
  [/^Stundenbericht (\S+) aus Harvest/, 'Work log $1 from Harvest'], // its name before 1.10
  [/^Rechnungsentwurf /, 'Invoice draft '],
  [/ · Zeitplan$/, ' · schedule'],
];
function title(t) {
  if (LANG !== 'en' || !t) return t;
  for (const [de, en] of TITLES) t = t.replace(de, en);
  return t;
}

// ── Cron ───────────────────────────────────────────────────────────────────

let schedule = null;

const scheduleJSON = () => JSON.stringify(schedule.entries.map(({ job, cron, enabled }) => ({ job, cron, enabled })));

async function loadCron() {
  if (schedule && scheduleJSON() !== schedule.saved) return; // changes not saved yet: kept
  try {
    schedule = await api('GET', '/api/schedule');
    schedule.saved = scheduleJSON();
  } catch (err) {
    $('schedule').replaceChildren(el('p', 'badge failed', err.message));
    return;
  }
  renderSchedule();
}
loaders.cron = loadCron;

const when = (iso) => new Date(iso).toLocaleString(LOCALE, { weekday: 'short', day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });

function renderSchedule() {
  const rows = schedule.entries.map((e, i) => {
    const job = el('select');
    job.append(...Object.entries(schedule.jobs).map(([name, jobTitle]) => {
      const o = el('option', '', title(jobTitle));
      o.value = name;
      return o;
    }));
    job.value = e.job;
    job.addEventListener('change', () => { e.job = job.value; });
    const cron = el('input');
    cron.value = e.cron;
    cron.spellcheck = false;
    cron.addEventListener('input', () => { e.cron = cron.value; });
    const on = el('input');
    on.type = 'checkbox';
    on.checked = e.enabled;
    on.addEventListener('change', () => { e.enabled = on.checked; });
    // One line: when it ran last and how, then the next runs, as many as fit.
    const last = el('span', 'muted', `${T('zuletzt', 'last')}: ${e.last ? lastRun(e.last) : T('noch nie', 'never')}`);
    const next = e.error ? el('span', 'badge failed', e.error)
      : el('span', `runs${e.enabled ? '' : ' muted'}`, e.enabled ? T('nächste: ', 'next: ') : T('aus — nächste wären: ', 'off — next would be: '),
        ...(e.next.length ? e.next.map((at, k) => el('span', 'run', `${k ? ' · ' : ''}${when(at)}`)) : ['—']));
    return el('div', `entry ${e.enabled ? '' : 'off'}`,
      el('div', 'entry-head',
        el('label', 'check', on, T('an', 'on')), job, cron,
        button(T('entfernen', 'remove'), 'link', () => { schedule.entries.splice(i, 1); renderSchedule(); refreshSave(); })),
      el('div', 'entry-line small', last, ' – ', next));
  });
  const output = el('div', 'console');
  output.dataset.console = 'cron';
  output.hidden = true;
  $('schedule').replaceChildren(...rows, output);
  fitRuns();
}

/** "Mo., 30.11.2026, 07:32 (manuell)"; the same day "vor 2 h (zeitgesteuert)". A failed run says so. */
function lastRun(job) {
  const at = never(job.ended) ? job.started : job.ended;
  const today = new Date(at).toDateString() === new Date().toDateString();
  const how = / · Zeitplan$/.test(job.title) ? T('zeitgesteuert', 'scheduled') : T('manuell', 'manual');
  return `${today ? ago(at) : when(at)} (${how})${job.status === 'failed' ? ` — ${STATUS().failed}` : ''}`;
}

/** The next runs on each entry's line: as many as fit whole, the rest left out. */
function fitRuns() {
  requestAnimationFrame(() => {
    for (const line of document.querySelectorAll('.entry-line')) {
      const runs = [...line.querySelectorAll('.run')];
      for (const run of runs) run.hidden = false;
      for (let k = runs.length - 1; k > 0 && line.scrollWidth > line.clientWidth; k--) runs[k].hidden = true;
    }
  });
}
window.addEventListener('resize', fitRuns);

$('schedule-add').addEventListener('click', () => {
  schedule.entries.push({ job: 'travel-expense', cron: '0 7 1 * *', enabled: false, next: [] });
  renderSchedule();
  refreshSave();
});

// ── Consoles: a job's live output where it was started ─────────────────────

/** Shows a job in a console box and follows it until it is done. */
async function follow(name, job, onDone) {
  const box = document.querySelector(`[data-console="${name}"]`);
  box.hidden = false;
  for (;;) {
    try {
      job = await api('GET', `/api/jobs/${job.id}`);
    } catch (err) {
      box.replaceChildren(el('p', 'badge failed', err.message));
      return;
    }
    renderJob(box, job);
    if (job.status !== 'running') break;
    await new Promise((r) => setTimeout(r, 1000));
  }
  loadOverview();
  onDone?.(job);
}

/** The text with its web addresses as links: Harvest's, for one, in a warning. */
function linked(text) {
  return text.split(/(https:\/\/[^\s,;)]+)/).map((part, i) => {
    if (i % 2 === 0) return part;
    const a = el('a', '', part);
    a.href = part;
    a.target = '_blank';
    a.rel = 'noopener';
    return a;
  });
}

function renderJob(box, job) {
  const pre = el('pre', '', ...linked((job.output ?? []).join('\n') || '…'));
  const files = (job.files ?? []).map((f) => {
    if (f.gone) return el('span', 'muted', `${f.name} (${T('nicht mehr verfügbar', 'no longer available')})`);
    const a = el('a', '', `${f.name} (${Math.round(f.size / 1024)} KB)`);
    a.href = `/api/jobs/${job.id}/files/${encodeURIComponent(f.name)}`;
    a.target = '_blank';
    return a;
  });
  box.replaceChildren(...[
    el('div', 'head', el('span', 'title', title(job.title)), el('span', `badge ${job.status}`, STATUS()[job.status] || job.status)),
    pre,
    job.error && el('p', 'badge failed', job.error),
    files.length > 0 && el('div', 'files', ...files),
  ].filter(Boolean));
  pre.scrollTop = pre.scrollHeight;
}

// A page left open across a restart of the server: its file links lead nowhere, so plain text.
document.addEventListener('visibilitychange', async () => {
  if (document.hidden) return;
  for (const a of document.querySelectorAll('.console .files a')) {
    const ok = await fetch(a.href, { method: 'HEAD' }).then((r) => r.ok, () => true);
    if (!ok) a.replaceWith(el('span', 'muted', `${a.textContent.replace(/ \(\d+ KB\)$/, '')} (${T('nicht mehr verfügbar', 'no longer available')})`));
  }
});

async function start(name, method, path, body, onDone) {
  try {
    const job = await api(method, path, body);
    loadOverview();
    follow(name, job, onDone);
  } catch (err) {
    alert(err.message);
  }
}

// Anthropic's and Apple's receipts by month: to look at, or sent after asking. The month as on
// the command line, the current one at first.
const NAMES = T({ anthropic: 'Anthropic-Belege', apple: 'Apple-Rechnungen', vodafone: 'Vodafone-Rechnungen' },
  { anthropic: 'Anthropic receipts', apple: 'Apple invoices', vodafone: 'Vodafone invoices' });
const monthOf = (kind) => document.querySelector(`.receipt-month[data-for="${kind}"]`).value;
{
  const now = new Date();
  for (const input of document.querySelectorAll('.receipt-month')) {
    input.value = input.placeholder = `${now.getMonth() + 1}/${now.getFullYear()}`;
  }
}
for (const b of document.querySelectorAll('[data-view]')) {
  b.addEventListener('click', () => start(b.dataset.view, 'POST', `/api/${b.dataset.view}/view`, { month: monthOf(b.dataset.view) }));
}
for (const b of document.querySelectorAll('[data-send]')) {
  b.addEventListener('click', () => {
    const kind = b.dataset.send;
    if (!confirm(T(`${NAMES[kind]} ${monthOf(kind)} senden an ${mailTo()}?`, `Send ${NAMES[kind]} ${monthOf(kind)} to ${mailTo()}?`))) return;
    start(kind, 'POST', `/api/${kind}/send`, { month: monthOf(kind) });
  });
}

// The month the Harvest API is asked for, as on the command line: the current one, or an
// earlier one after the fact.
// Not restored by the browser on reloading — an emptied field came back empty over it — and the
// hint the same month, should it be emptied.
{
  const now = new Date();
  const month = `${now.getMonth() + 1}/${now.getFullYear()}`;
  $('harvest-month').value = month;
  $('harvest-month').placeholder = month;
}
$('harvest-run').addEventListener('click', () =>
  start('harvest', 'POST', '/api/harvest/run', { month: $('harvest-month').value }));
$('harvest-worklog').addEventListener('click', () =>
  start('harvest', 'POST', '/api/harvest/worklog', { month: $('harvest-month').value }));

// ── Travel expenses ────────────────────────────────────────────────────────

let month = new Date();
month = new Date(month.getFullYear(), month.getMonth(), 1);
let plan = null;

const monthKey = () => `${month.getFullYear()}-${String(month.getMonth() + 1).padStart(2, '0')}`;

$('prev-month').addEventListener('click', () => { month.setMonth(month.getMonth() - 1); loadTravel(); });
$('next-month').addEventListener('click', () => { month.setMonth(month.getMonth() + 1); loadTravel(); });

async function loadTravel() {
  $('month-title').textContent = `${MONTHS[month.getMonth()]} ${month.getFullYear()}`;
  try {
    plan = await api('GET', `/api/travel/plan?month=${monthKey()}`);
  } catch (err) {
    $('calendar').replaceChildren(el('p', 'badge failed', err.message));
    return;
  }
  if (!overview) await loadOverview();
  takeLists();
  renderCalendar();
  renderTotals();
  renderRanges();
}
loaders.travel = () => { loadTravel(); loadSettings(); };

const KIND_TEXT = T({ weekend: '', holiday: 'Feiertag', christmas: 'Weihnachten', blocked: 'gesperrt', home: 'kein Kunde' },
  { weekend: '', holiday: 'holiday', christmas: 'Christmas', blocked: 'blocked', home: 'no client' });

// Pixel icons, 16 × 16, each centred in its square: what an amount is for — fork and knife, a
// car, a bed. Drawn by pixel,
// '#' the outline, '+' the shade, '.' the light: shades of the text's colour, so that they turn
// light on dark with the theme.
const GRIDS = {
  meal: [
    '                ',
    '   # # #   ##   ',
    '   #.#.#  #.#   ',
    '   #.#.# #..#   ',
    '   #.#.# #..#   ',
    '   #...# #..#   ',
    '    #+#  #..#   ',
    '    #+#  #..#   ',
    '    #+#  ##+#   ',
    '    #+#   #+#   ',
    '    #+#   #+#   ',
    '    #+#   #+#   ',
    '    #+#   #+#   ',
    '    #+#   #+#   ',
    '     ##    ##   ',
    '                ',
  ],
  car: [
    '                ',
    '                ',
    '                ',
    '     ######     ',
    '    ##..#.##    ',
    '    #...#..##   ',
    '  ###...#...##  ',
    '.#++++++++++++#.',
    '#+++++++++++++.#',
    '#.+###++++###+.#',
    '#+##..#++#..##+#',
    ' ###..####..### ',
    '   ###    ###   ',
    '                ',
    '                ',
    '                ',
  ],
  bed: [
    '                ',
    '                ',
    '                ',
    '##              ',
    '##              ',
    '#####           ',
    '##...#          ',
    '##...######## ##',
    '##+++++++++++###',
    '##++++++++++++##',
    '################',
    '################',
    '##            ##',
    '                ',
    '                ',
    '                ',
  ],
  // The night flight home: a plane seen from above, nose to the right.
  plane: [
    '                ',
    '      #         ',
    '      ##        ',
    '       ##       ',
    '       ###      ',
    '##      ###     ',
    '###     ####    ',
    ' ##############.',
    ' ##############.',
    '###     ####    ',
    '##      ###     ',
    '       ###      ',
    '       ##       ',
    '      ##        ',
    '      #         ',
    '                ',
  ],
};
const SHADES = { '#': 1, '+': 0.55, '.': 0.28 };

// The moon on a day away, 8 × 8 and solid, a marker in the corner: rectangles x y width height.
const MOON = [[2, 0, 4, 1], [1, 1, 3, 1], [0, 2, 3, 4], [1, 6, 3, 1], [2, 7, 4, 1], [6, 6, 1, 1]];

/** A pixel icon, its meaning as its title: meal, car, bed and plane of GRIDS, or the moon. */
function icon(name, title) {
  const ns = 'http://www.w3.org/2000/svg';
  const svg = document.createElementNS(ns, 'svg');
  const rect = (x, y, w, h, shade) => {
    const r = document.createElementNS(ns, 'rect');
    for (const [k, v] of Object.entries({ x, y, width: w, height: h })) r.setAttribute(k, v);
    if (shade < 1) r.setAttribute('fill-opacity', shade);
    svg.append(r);
  };
  svg.setAttribute('role', 'img');
  svg.setAttribute('aria-label', title);
  const t = document.createElementNS(ns, 'title');
  t.textContent = title;
  svg.append(t);
  if (name === 'moon') {
    svg.setAttribute('viewBox', '0 0 8 8');
    svg.setAttribute('class', 'px px-moon');
    for (const [x, y, w, h] of MOON) rect(x, y, w, h, 1);
    return svg;
  }
  svg.setAttribute('viewBox', '0 0 16 16');
  svg.setAttribute('class', `px px16 px-${name}`);
  // A run of pixels of one shade in a row is one rectangle.
  GRIDS[name].forEach((row, y) => {
    for (let x = 0; x < row.length;) {
      const c = row[x];
      let end = x + 1;
      while (end < row.length && row[end] === c) end++;
      if (SHADES[c]) rect(x, y, end - x, 1, SHADES[c]);
      x = end;
    }
  });
  return svg;
}

// The page's own icons, as the Zuordnungen' checkboxes have them: <span data-icon="moon" title="…">.
for (const s of document.querySelectorAll('[data-icon]')) s.replaceWith(icon(s.dataset.icon, s.title));

/** An amount after its icon: "🍴 44,00 €". */
const amount = (name, title, eur) => el('span', 'amount', icon(name, title), euro(eur));

function renderCalendar() {
  const cells = T(['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So'], ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']).map((d) => el('div', 'wd', d));
  const first = (new Date(`${plan.month}-01T00:00:00`).getDay() + 6) % 7;
  for (let i = 0; i < first; i++) cells.push(el('div', 'day pad'));
  const today = new Date().toISOString().slice(0, 10);
  for (const d of plan.days) {
    const classes = ['day', d.kind, d.clientNo && colourOf(d.clientNo), d.assigned && 'assigned', d.date === today && 'today', (d.missing || d.lodgingMissing) && 'fallback'].filter(Boolean).join(' ');
    const who = d.client ? el('span', 'who', d.client) : el('span', 'note', d.note || KIND_TEXT[d.kind] || '');
    // What the day is worth, each amount after its icon: meals, the drive, the night.
    const eur = d.meal || d.mileage || d.lodging ? el('span', 'eur',
      d.meal && amount('meal', W.meals, d.meal),
      d.mileage && amount('car', W.mileage, d.mileage),
      d.lodging && amount('bed', W.night, d.lodging)) : null;
    const cell = el('div', classes, el('span', 'n', String(Number(d.date.slice(8)))), d.kind === 'away' && el('span', 'moon', icon('moon', W.overnight)), who,
      d.client && d.note && el('span', 'note muted', d.note), eur);
    cell.title = [dmy(d.date), d.client, d.mealText && `${W.meals} ${mealText(d.mealText)}`, d.lodging && `${W.night} ${euro(d.lodging)}`, d.note,
      d.missing && T(`Satz „${RATE_NAMES[d.missing]}“ des Kunden fehlt — Inlandssatz ${euro(d.meal)}`, `The client's rate “${RATE_NAMES[d.missing]}” is missing — inland rate ${euro(d.meal)}`),
      d.lodgingMissing && T(`Satz „${RATE_NAMES.lodging}“ des Kunden fehlt — Inlandssatz ${euro(d.lodging)}`, `The client's rate “${RATE_NAMES.lodging}” is missing — inland rate ${euro(d.lodging)}`)].filter(Boolean).join(' · ');
    cells.push(cell);
  }
  $('calendar').replaceChildren(...cells);
  // The legend: each client in its colour.
  $('legend-clients').replaceChildren(...(plan.clients ?? []).map((name, i) => el('span', `k ${colourOf(i + 1)}`, name)));
  $('legend-icons').replaceChildren(
    el('span', '', icon('moon', W.overnight), W.overnight),
    el('span', '', icon('meal', W.meals), W.meals),
    el('span', '', icon('car', W.mileage), W.mileage),
    el('span', '', icon('bed', W.allowance), W.allowance));
}

/** The words for what a travel day is worth, and how it is spent. */
const W = T(
  { overnight: 'Übernachtung', meals: 'Verpflegung', mileage: 'Kilometergeld', night: 'Übernachtung', allowance: 'Übernachtungspauschale', days: 'Tagesreisen' },
  { overnight: 'Overnight', meals: 'Meals', mileage: 'Mileage', night: 'Night', allowance: 'Overnight allowance', days: 'Day trips' });

/** The day's meal rate as the server names it: "8h - 24h", "24h", "An-/Abreisetag". */
const mealText = (text) => (text === 'An-/Abreisetag' ? T(text, 'arrival/departure day') : text);

/** A client's colour class, by its place in the list from 1: six colours, then again. */
const colourOf = (no) => `c${((no - 1) % 6) + 1}`;

const RATE_NAMES = T({ mealDay: 'Tagesreise > 8 h', mealFullDay: 'Ganzer Tag', mealTravelDay: 'An-/Abreisetag', lodging: 'Übernachtung' },
  { mealDay: 'Day trip > 8 h', mealFullDay: 'Full day', mealTravelDay: 'Arrival/departure day', lodging: 'Night' });

/** What the month's expenses come to: meals, mileage and nights. */
const totalOf = (t) => (t.meals ?? 0) + (t.mileage ?? 0) + (t.lodging ?? 0);

function renderTotals() {
  const t = plan.totals;
  // Clients abroad falling back to the inland rate: named once each, under the totals.
  const missing = new Map();
  for (const d of plan.days) {
    if (d.missing) missing.set(`${d.client}|${d.missing}`, T(`${d.client}: Satz „${RATE_NAMES[d.missing]}“ fehlt — es gilt der Inlandssatz ${euro(d.meal)}`, `${d.client}: rate “${RATE_NAMES[d.missing]}” missing — the inland rate applies, ${euro(d.meal)}`));
    if (d.lodgingMissing) missing.set(`${d.client}|lodging`, T(`${d.client}: Satz „${RATE_NAMES.lodging}“ fehlt — es gilt der Inlandssatz ${euro(d.lodging)}`, `${d.client}: rate “${RATE_NAMES.lodging}” missing — the inland rate applies, ${euro(d.lodging)}`));
  }
  const per = Object.entries(plan.perClient).map(([name, n]) => `${name} ${n}`).join(' · ');
  $('travel-totals').replaceChildren(...[
    el('div', '', el('dt', '', T('Reisetage', 'Travel days')), el('dd', '', String(t.days ?? 0))),
    el('div', '', el('dt', '', icon('meal', W.meals), W.meals), el('dd', '', euro(t.meals))),
    el('div', '', el('dt', '', icon('car', W.mileage), T('Kilometer', 'Mileage')), el('dd', '', euro(t.mileage))),
    t.nights > 0 && el('div', '', el('dt', '', icon('bed', W.night), T('Übernachtung', 'Nights')), el('dd', '', euro(t.lodging))),
    el('div', '', el('dt', '', T('Gesamt', 'Total')), el('dd', '', euro(totalOf(t)))),
    per && el('div', 'whole', el('dt', '', T('Je Kunde', 'Per client')), el('dd', 'muted', per)),
    missing.size > 0 && el('div', 'whole warn', el('dt', '', T('Satz fehlt', 'Rate missing')),
      el('dd', '', ...[...missing.values()].flatMap((text, i) => (i ? [el('br'), text] : [text])), el('br'),
        T('Beim Kunden eintragen: ', 'Enter it with the client: '), bmfLink())),
  ].filter(Boolean));
}

function renderRanges() {
  const clients = overview?.clients ?? [];
  const select = $('a-client');
  const chosen = select.value;
  // No client chosen to begin with: an assignment is made on purpose, not by default.
  const none = el('option', '', T('Kunde wählen…', 'Choose client…'));
  none.value = '';
  select.replaceChildren(none, ...clients.map((c) => {
    const o = el('option', '', c.name);
    o.value = c.id || c.name;
    return o;
  }));
  select.value = chosen;
  renderLists();
}

// Zuordnungen and gesperrte Tage are edited as drafts: adding, changing and removing touch only
// the list, the Speichern button at the bottom saves it. Each list: its draft, the entry being
// edited in the form, and whether the draft differs from what is saved.
const lists = {
  assignments: { draft: null, editing: -1, dirty: false, path: '/api/travel/assignments', form: 'assignment-form', add: T('Zuordnen', 'Assign') },
  blocked: { draft: null, editing: -1, dirty: false, path: '/api/travel/blocked', form: 'blocked-form', add: T('Sperren', 'Block') },
};

/** Takes the saved lists as drafts, unless one has changes not saved yet. */
function takeLists() {
  for (const [name, list] of Object.entries(lists)) {
    if (!list.dirty) list.draft = structuredClone(plan[name]);
  }
}

function renderLists() {
  const clients = overview?.clients ?? [];
  const nameOf = (key) => clients.find((c) => c.id === key || c.name === key)?.name ?? key;
  const span = (r) => (r.to && r.to !== r.from ? `${dmy(r.from)} – ${dmy(r.to)}` : dmy(r.from));
  const what = {
    // With the form's icons: moon, bed, plane; the car for day trips.
    assignments: (a) => [nameOf(a.client),
      // Overnight with a hotel invoice: Hotel says it, Übernachtung beside it would say it twice.
      ...(!a.overnight ? [' · ', icon('car', W.days), W.days]
        : a.hotel ? [' · ', icon('bed', 'Hotel'), 'Hotel']
          : [' · ', icon('moon', W.overnight), W.overnight]),
      ...(a.overnight && a.nightFlight ? [' · ', icon('plane', T('Nachtflug', 'Night flight')), T('Nachtflug', 'Night flight')] : []),
      a.note ? ` · ${a.note}` : ''],
    blocked: (b) => [b.note || ''],
  };
  for (const [name, list] of Object.entries(lists)) {
    const saved = JSON.stringify(plan[name]);
    list.dirty = JSON.stringify(list.draft) !== saved;
    $(name).replaceChildren(...list.draft.map((entry, i) => el('li', [
      list.editing === i && 'editing',
      !plan[name].some((s) => JSON.stringify(s) === JSON.stringify(entry)) && 'changed',
    ].filter(Boolean).join(' '),
      el('span', 'when', span(entry)),
      el('span', 'grow', ...what[name](entry)),
      button(T('bearbeiten', 'edit'), 'link', () => editEntry(name, i)),
      button(T('entfernen', 'remove'), 'link', () => {
        list.draft.splice(i, 1);
        if (list.editing === i) stopEditing(name);
        else if (list.editing > i) list.editing--;
        renderLists();
      }))));
    const form = $(list.form);
    form.querySelector('button[type=submit]').textContent = list.editing >= 0 ? T('Übernehmen', 'Apply') : list.add;
    form.querySelector('.cancel').hidden = list.editing < 0;
  }
  refreshSave();
}

/** Puts an entry back into its form, to change it and take it over. */
function editEntry(name, i) {
  const list = lists[name];
  const e = list.draft[i];
  list.editing = i;
  if (name === 'assignments') {
    $('a-client').value = e.client;
    $('a-from').value = e.from;
    $('a-to').value = e.to || '';
    $('a-overnight').checked = !!e.overnight;
    $('a-hotel').checked = !!e.hotel;
    $('a-flight').checked = !!e.nightFlight;
    syncHotel();
    $('a-note').value = e.note || '';
  } else {
    $('b-from').value = e.from;
    $('b-to').value = e.to || '';
    $('b-note').value = e.note || '';
  }
  for (const id of ['a-from', 'b-from']) $(id).dispatchEvent(new Event('change'));
  renderLists();
  $(list.form).scrollIntoView({ block: 'center', behavior: 'smooth' });
}

function stopEditing(name) {
  lists[name].editing = -1;
  $(lists[name].form).reset();
  if (name === 'assignments') syncHotel();
}

/** The form's entry, added to the draft or taking the place of the one being edited. */
function takeForm(name, entry) {
  const list = lists[name];
  if (list.editing >= 0) list.draft[list.editing] = entry;
  else list.draft.push(entry);
  stopEditing(name);
  renderLists();
}

$('assignment-form').addEventListener('submit', (e) => {
  e.preventDefault();
  takeForm('assignments', {
    client: $('a-client').value, from: $('a-from').value, to: $('a-to').value,
    overnight: $('a-overnight').checked, hotel: $('a-overnight').checked && $('a-hotel').checked,
    nightFlight: $('a-overnight').checked && $('a-flight').checked,
    note: $('a-note').value.trim(),
  });
  syncHotel();
});

/**
 * "Hotelrechnung" and "Nachtflug" are for overnight trips only: their nights at
 * the flat rate, but for a hotel invoice, and for the night on the plane home.
 */
function syncHotel() {
  for (const id of ['a-hotel-label', 'a-flight-label']) $(id).hidden = !$('a-overnight').checked;
}
$('a-overnight').addEventListener('change', syncHotel);
syncHotel();

$('blocked-form').addEventListener('submit', (e) => {
  e.preventDefault();
  takeForm('blocked', { from: $('b-from').value, to: $('b-to').value, note: $('b-note').value.trim() });
});

for (const name of Object.keys(lists)) {
  $(lists[name].form).querySelector('.cancel').addEventListener('click', () => { stopEditing(name); renderLists(); });
}

// Date ranges: "von" starts at today, "bis" only from the day after "von" on — empty for a
// single day. Kept so after every reset of the form.
const isoDay = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
for (const [from, to] of [['a-from', 'a-to'], ['b-from', 'b-to']]) {
  const follow = () => {
    if (!$(from).value) return;
    const next = new Date(`${$(from).value}T12:00:00`);
    next.setDate(next.getDate() + 1);
    $(to).min = isoDay(next);
    if ($(to).value && $(to).value < $(to).min) $(to).value = '';
  };
  $(from).defaultValue = isoDay(new Date());
  $(from).value = $(from).defaultValue;
  $(from).addEventListener('change', follow);
  $(from).form.addEventListener('reset', () => setTimeout(follow));
  follow();
}

// Leaving with changes not saved: the browser asks first.
window.addEventListener('beforeunload', (e) => {
  if (unsaved().length) e.preventDefault();
});

$('travel-generate').addEventListener('click', () =>
  start('travel', 'POST', '/api/travel/generate', { month: monthKey() }));
$('travel-send').addEventListener('click', () => {
  const t = plan?.totals ?? {};
  if (!confirm(T(`Reisekosten ${MONTHS[month.getMonth()]} ${month.getFullYear()} senden an ${mailTo()}?\n${t.days ?? 0} Reisetage, ${euro(totalOf(t))}`,
    `Send the travel expenses of ${MONTHS[month.getMonth()]} ${month.getFullYear()} to ${mailTo()}?\n${t.days ?? 0} travel days, ${euro(totalOf(t))}`))) return;
  start('travel', 'POST', '/api/travel/send', { month: monthKey() });
});

// ── Customer invoice ───────────────────────────────────────────────────────

let upload = null;
const drop = $('upload-form');
drop.addEventListener('dragover', (e) => { e.preventDefault(); drop.classList.add('over'); });
drop.addEventListener('dragleave', () => drop.classList.remove('over'));
drop.addEventListener('drop', (e) => {
  e.preventDefault();
  drop.classList.remove('over');
  if (e.dataTransfer.files[0]) sendReport(e.dataTransfer.files[0]);
});
$('report').addEventListener('change', () => { if ($('report').files[0]) sendReport($('report').files[0]); });

async function sendReport(file) {
  $('drop-text').textContent = T(`${file.name} wird gelesen…`, `Reading ${file.name}…`);
  const form = new FormData();
  form.append('report', file);
  try {
    upload = await api('POST', '/api/harvest/upload', form);
  } catch (err) {
    $('drop-text').textContent = err.message;
    return;
  }
  $('drop-text').textContent = T(`${file.name}: ${String(upload.hours).replace('.', ',')} Stunden gelesen`, `${file.name}: ${upload.hours} hours read`);
  $('d-hours').value = upload.hours;
  const now = new Date();
  $('d-from').value = upload.from || `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-01`;
  $('d-to').value = upload.to || new Date(now.getFullYear(), now.getMonth() + 1, 0, 12).toISOString().slice(0, 10);
  $('d-client').value = upload.client || T('— in den Einstellungen setzen —', '— set it in the settings —');
  $('d-product').value = upload.product;
  $('d-reference').value = upload.reference;
  $('draft-form').hidden = false;
}

$('draft-form').addEventListener('submit', (e) => {
  e.preventDefault();
  const hours = Number($('d-hours').value);
  if (!confirm(T(`Rechnungsentwurf in sevDesk anlegen?\n${upload.client}: ${String(hours).replace('.', ',')} Stunden, ${dmy($('d-from').value)} – ${dmy($('d-to').value)}`,
    `Create the invoice draft in sevDesk?\n${upload.client}: ${hours} hours, ${dmy($('d-from').value)} – ${dmy($('d-to').value)}`))) return;
  start('invoice', 'POST', '/api/harvest/draft', { id: upload.id, hours, from: $('d-from').value, to: $('d-to').value });
});

// ── Log ────────────────────────────────────────────────────────────────────

// The open run's output, under its entry; a second click closes it.
const logConsole = el('div', 'console');
logConsole.dataset.console = 'log';
let openJob = null;

async function loadLog() {
  const jobs = await api('GET', '/api/jobs').catch(() => []);
  $('no-jobs').hidden = jobs.length > 0;
  $('clear-log').disabled = jobs.length === 0;
  $('jobs').replaceChildren(...jobs.flatMap((job) => {
    const li = el('li', '',
      el('span', 'when', new Date(job.started).toLocaleString(LOCALE, { dateStyle: 'short', timeStyle: 'short' })),
      el('span', 'grow title', title(job.title)),
      el('span', `badge ${job.status}`, STATUS()[job.status] || job.status));
    li.addEventListener('click', () => {
      $('jobs').querySelector('li.detail')?.remove();
      for (const other of $('jobs').children) other.classList.remove('on');
      if (openJob === job.id) {
        openJob = null;
        return;
      }
      openJob = job.id;
      li.classList.add('on');
      li.after(el('li', 'detail', logConsole));
      follow('log', job, () => {});
    });
    if (job.id !== openJob) return [li];
    li.classList.add('on');
    return [li, el('li', 'detail', logConsole)];
  }));
}
loaders.log = loadLog;

// Löschen: the runs gone, their files with them — asked once more, as Speichern asks.
let clearArmed = null;
function disarmClear() {
  clearTimeout(clearArmed);
  clearArmed = null;
  $('clear-log').classList.remove('armed');
  $('clear-log').replaceChildren(T('Löschen', 'Delete'));
}
$('clear-log').addEventListener('click', async () => {
  if (!clearArmed) {
    $('clear-log').classList.add('armed');
    $('clear-log').textContent = T('Sicher?', 'Sure?');
    clearArmed = setTimeout(disarmClear, 4000);
    return;
  }
  disarmClear();
  try {
    await api('DELETE', '/api/jobs');
  } catch (err) {
    alert(err.message);
    return;
  }
  openJob = null;
  loadLog();
  loadOverview();
});

// ── Settings ───────────────────────────────────────────────────────────────

/**
 * The BMF's table of meal and overnight rates abroad, for the year of the month shown: the
 * server finds that year's in the Lohnsteuer-Handbuch, or the year before's while it is not out
 * yet — no link to move on each year.
 */
function bmfLink() {
  const year = plan?.month ? Number(plan.month.slice(0, 4)) : new Date().getFullYear();
  const a = el('a', '', T('BMF-Ländertabelle', "BMF's table of countries"));
  a.href = `/api/bmf-table?year=${year}`;
  a.target = '_blank';
  a.rel = 'noopener';
  return a;
}

const PROVINCES = ['BW', 'BY', 'BE', 'BB', 'HB', 'HH', 'HE', 'MV', 'NI', 'NW', 'RP', 'SL', 'SN', 'ST', 'SH', 'TH'];
// A new line in a form's grid: what follows starts on the left.
const BREAK = ['', '', 'break'];

// Section titles the code looks for, and labels used twice.
const CLIENTS = T('Kunden', 'Clients');
const INLAND = T('Pauschalen Inland', 'Inland rates');
const SCAN = T('Mails durchsuchen (je Ordner)', 'Mails to search (per folder)');
const FOLDERS = T('Ordner, der Reihe nach', 'Folders, in order');
const DEFAULT = T('Standard', 'default');

// The settings as the config file has them, each where it is used: per part of the page, its
// sections with field path, label, type. Only what all of them share is under Einstellungen.
const PARTS = {
  company: [[T('Firma', 'Company'), [['Company.Name', T('Firmenname', 'Company name')]]]],
  staff: [[T('Mitarbeiter', 'Employees'), []]],
  mail: [[T('E-Mail', 'E-mail'), [
    ['Mail.SMTPHost', 'SMTP-Server'], ['Mail.SMTPPort', 'SMTP-Port', 'number'],
    ['Mail.IMAPHost', 'IMAP-Server'], ['Mail.IMAPPort', 'IMAP-Port', 'number'],
    ['Mail.Username', T('Benutzer', 'User')], ['Mail.Password', T('Passwort', 'Password'), 'secret', 0],
    ['Mail.From', T('Absender', 'Sender')], ['Mail.To', T('Empfänger', 'Recipient')], ['Mail.CC', T('Kopie (z. B. sevDesk-Autobox)', 'Copy (e.g. sevDesk Autobox)')],
  ]]],
  web: [[T('Oberfläche', 'Web interface'), [
    ['Web.Listen', T('Adresse (Neustart nötig)', 'Address (needs a restart)'), 'text', T('localhost:8595, auch im Netz (:8595)', 'localhost:8595, on the network too (:8595)')],
    ['Web.HomeOnly', T('Nur im Heimnetz erreichbar', 'Home network only'), 'bool', true],
    ['Web.ProxyToken', T('Proxy-Token (Caddy in Docker)', 'Proxy token (Caddy in Docker)'), 'secret', 4],
  ]]],
  travel: [[CLIENTS, [
    ['TravelExpense.ChristmasWeekOff', T('Weihnachtswoche frei (24. und 27.–31.12.)', 'Christmas week off (Dec 24 and 27–31)'), 'bool', true],
  ]], [INLAND, [
    ['TravelExpense.MealRates.Day', `${RATE_NAMES.mealDay} (€)`, 'money', 14],
    ['TravelExpense.MealRates.FullDay', `${RATE_NAMES.mealFullDay} (€)`, 'money', 28],
    ['TravelExpense.MealRates.TravelDay', `${RATE_NAMES.mealTravelDay} (€)`, 'money', 14],
    ['TravelExpense.LodgingRate', `${RATE_NAMES.lodging} (€)`, 'money', 20],
  ]]],
  invoice: [['', [
    ['HarvestInvoice.SevDesk.ClientName', T('Kunde in sevDesk', 'Client in sevDesk')],
    ['HarvestInvoice.SevDesk.ProductName', T('Artikel (Suchbegriff)', 'Product (search term)')], ['HarvestInvoice.SevDesk.ProductNum', T('Artikelnummer', 'Product number')],
    ['HarvestInvoice.SevDesk.ReferenceNum', T('Kundenreferenz (E-Rechnung)', 'Client reference (e-invoice)')],
    BREAK,
    ['HarvestInvoice.SevDesk.Username', T('sevDesk-Benutzer', 'sevDesk user')], ['HarvestInvoice.SevDesk.Password', T('sevDesk-Passwort', 'sevDesk password'), 'secret', 2],
    BREAK,
    ['HarvestInvoice.Harvest.Token', T('Harvest-API-Token', 'Harvest API token'), 'secret', 3],
    ['HarvestInvoice.Harvest.Client', T('Kunde in Harvest', 'Client in Harvest'), 'shadow', 'HarvestInvoice.SevDesk.ClientName'],
    BREAK,
    ['HarvestInvoice.SkipExisting', T('Nicht doppelt anlegen (sevDesk prüfen)', 'Never twice (check sevDesk)'), 'bool', true],
  ]]],
  anthropic: [['', [
    ['AnthropicInvoice.Filter.Count', SCAN, 'number', 50],
    ['AnthropicInvoice.Filter.From', T('Absender-Domains', 'Sender domains'), 'text', 'mail.anthropic.com, stripe.com'],
    ['AnthropicInvoice.Filter.Subject', T('Betreff beginnt mit', 'Subject starts with'), 'text', 'Your receipt from Anthropic'],
    ['AnthropicInvoice.Filter.Folders', FOLDERS, 'list', 'INBOX, Rechnungen'],
  ]]],
  apple: [['', [
    ['AppleInvoice.Filter.Count', SCAN, 'number', 50],
    ['AppleInvoice.Filter.From', T('Absender-Domain', 'Sender domain'), 'text', 'apple.com'],
    ['AppleInvoice.Filter.Subject', T('Betreff', 'Subject'), 'text', 'Deine Rechnung von Apple'],
    ['AppleInvoice.Filter.Folders', FOLDERS, 'list', 'INBOX, Rechnungen'],
  ]]],
  vodafone: [['', [
    ['VodafoneInvoice.Username', T('Benutzer', 'User')], ['VodafoneInvoice.Password', T('Passwort', 'Password'), 'secret', 1],
    ['VodafoneInvoice.FallbackToLastMonth', T('Sonst die Rechnung vom Vormonat', 'Else the previous month\'s invoice'), 'bool', true],
  ]]],
};
const CLIENT_FIELDS = [
  ['ID', T('Nr.', 'No.')], ['Name', 'Name'], ['From', T('Von', 'From')], ['To', T('Nach', 'To')], ['Reason', T('Grund', 'Purpose')],
  ['Distance', T('Entfernung (km, einfach)', 'Distance (km, one way)'), 'number'], ['Country', T('Land', 'Country'), 'country'], ['Province', T('Bundesland', 'State'), 'province'],
  ['MealDay', `${RATE_NAMES.mealDay} (€)`, 'money', 14], ['MealFullDay', `${RATE_NAMES.mealFullDay} (€)`, 'money', 28],
  ['MealTravelDay', `${RATE_NAMES.mealTravelDay} (€)`, 'money', 14], ['Lodging', `${RATE_NAMES.lodging} (€)`, 'money', 20],
];

let settings = null;

const getPath = (obj, path) => path.split('.').reduce((o, k) => o?.[k], obj);
function setPath(obj, path, value) {
  const keys = path.split('.');
  const last = keys.pop();
  keys.reduce((o, k) => (o[k] ??= {}), obj)[last] = value;
}

function field(obj, [path, label, type, extra]) {
  if (type === 'break') return el('div', 'break');
  const value = getPath(obj, path);
  let input;
  if (type === 'bool') {
    input = el('input');
    input.type = 'checkbox';
    input.checked = value ?? extra;
    bind(input, () => setPath(obj, path, input.checked));
    return el('label', 'check span', input, label);
  }
  if (type === 'province') {
    input = el('select');
    input.append(...PROVINCES.map((p) => el('option', '', p)));
    input.value = value || 'BW';
  } else if (type === 'country') {
    input = el('select');
    // In English the countries' English names, from the browser.
    const english = LANG === 'en' && typeof Intl.DisplayNames === 'function' ? new Intl.DisplayNames(['en'], { type: 'region' }) : null;
    const names = Object.entries(settings.countries).map(([code, name]) => [code, english?.of(code) ?? name]).sort((a, b) => a[1].localeCompare(b[1], LANG));
    input.append(...names.map(([code, name]) => {
      const o = el('option', '', `${name} (${code})`);
      o.value = code;
      return o;
    }));
    input.value = (value || 'DE').toUpperCase();
    // Germany has states, other countries their national holidays and their own rates.
    input.addEventListener('change', () => {
      setPath(obj, path, input.value === 'DE' ? '' : input.value);
      if (input.value !== 'DE') obj.Province = '';
      renderSettings();
    });
    return el('label', '', label, input);
  } else {
    input = el('input');
    input.type = type === 'secret' ? 'password' : type === 'number' || type === 'money' ? 'number' : 'text';
    input.dataset.path = path;
    if (type === 'money') {
      input.step = '0.01';
      // Abroad, an empty rate falls back to the inland one: marked.
      const stand = { MealDay: 'MealTravelDay', MealTravelDay: 'MealDay' }[path];
      const client = path.startsWith('Meal') || path === 'Lodging';
      if (client && (obj.Country || 'DE').toUpperCase() !== 'DE' && !value && !(stand && obj[stand])) input.classList.add('missing');
      if (path === 'Lodging') {
        // A night without a hotel invoice: the client's rate, else the inland one — abroad the
        // country's from the BMF's table is wanted.
        const inland = getPath(settings.config, 'TravelExpense.LodgingRate') || extra;
        input.placeholder = (obj.Country || 'DE').toUpperCase() === 'DE' ? T(`leer: ${inland} (Inland)`, `empty: ${inland} (inland)`) : T('Satz des Landes', "the country's rate");
      } else if (path.startsWith('Meal')) {
        // A client's own rate: else the inland one — abroad that is wrong, the country's is wanted.
        const inland = getPath(settings.config, `TravelExpense.MealRates.${path.slice(4)}`) || extra;
        // Abroad, the day trip and travel day rates are one in the BMF's table: each stands in
        // for the other.
        const other = { MealDay: 'MealTravelDay', MealTravelDay: 'MealDay' }[path];
        input.placeholder = (obj.Country || 'DE').toUpperCase() === 'DE' ? T(`leer: ${inland} (Inland)`, `empty: ${inland} (inland)`)
          : other && obj[other] ? T(`leer: ${obj[other]} (wie ${path === 'MealDay' ? 'An-/Abreisetag' : 'Tagesreise'})`, `empty: ${obj[other]} (as ${path === 'MealDay' ? 'arrival/departure day' : 'day trip'})`)
          : T('Satz des Landes', "the country's rate");
      } else {
        input.placeholder = T(`Standard: ${extra} (gesetzlich)`, `default: ${extra} (legal)`);
      }
    }
    input.value = type === 'secret' ? '' : type === 'list' ? (value ?? []).join(', ')
      : value || (type === 'number' || type === 'money' ? '' : value ?? '');
    if (type === 'secret') input.placeholder = settings.secretsSet[extra] ? T('gespeichert — zum Ändern eingeben', 'saved — type to change') : '';
    // Left empty, the default applies: shown as the hint.
    if ((type === 'text' || type === 'number' || type === 'list') && extra !== undefined) input.placeholder = `${DEFAULT}: ${extra}`;
    // Left empty, another field's value applies: shown as the hint, as it is typed there.
    if (type === 'shadow') {
      input.dataset.shadowOf = extra;
      input.placeholder = getPath(obj, extra) ? `${DEFAULT}: ${getPath(obj, extra)}` : '';
    }
    input.autocomplete = 'off';
  }
  // A list is written with commas between: "INBOX, Rechnungen".
  const valueOf = () => (type === 'number' || type === 'money' ? Number(input.value) || 0
    : type === 'list' ? input.value.split(',').map((s) => s.trim()).filter(Boolean) : input.value);
  bind(input, () => setPath(obj, path, valueOf()));
  return el('label', '', label, input);
}

document.addEventListener('input', (e) => {
  const path = e.target.dataset && e.target.dataset.path;
  if (!path) return;
  for (const shadow of document.querySelectorAll(`[data-shadow-of="${path}"]`)) shadow.placeholder = e.target.value ? `${DEFAULT}: ${e.target.value}` : '';
});

function bind(input, update) {
  input.addEventListener('input', update);
  input.addEventListener('change', update);
  return input;
}

async function loadSettings() {
  if (settings && JSON.stringify(settings.config) !== settings.saved) return; // changes not saved yet: kept
  try {
    settings = await api('GET', '/api/settings');
    settings.saved = JSON.stringify(settings.config);
  } catch (err) {
    for (const form of document.querySelectorAll('form.part')) form.replaceChildren(el('p', 'badge failed', err.message));
    return;
  }
  renderSettings();
}
for (const tab of ['settings', 'invoice', 'receipts']) loaders[tab] = loadSettings;

/** Fills every settings part on the page from the one configuration they all edit. */
function renderSettings() {
  if (!settings) return;
  const cfg = settings.config;
  for (const form of document.querySelectorAll('form.part')) {
    const part = form.dataset.part;
    const children = [];
    for (const [title, fields] of PARTS[part]) {
      if (title) children.push(el('h2', '', title));
      if (title === INLAND) {
        children.push(el('p', 'muted', T('Für Kunden ohne eigene Sätze. Leer: die gesetzlichen Pauschalen (Verpflegung 14 / 28 / 14 €, Übernachtung ohne Hotelrechnung 20 €). Auslandssätze: ',
          'For clients without their own rates. Empty: the legal allowances (meals 14 / 28 / 14 €, a night without a hotel invoice 20 €). Rates abroad: '), bmfLink(), '.'));
      }
      if (title === CLIENTS) children.push(renderClients(cfg));
      if (part === 'staff') children.push(renderStaff(cfg));
      if (fields.length) children.push(el('fieldset', 'grid', ...fields.map((f) => field(cfg, f))));
      if (part === 'company') children.push(renderLogo());
    }
    if (part === 'web') {
      children.push(el('p', 'muted', T('Nur im Heimnetz: nur Geräte mit privater Adresse (192.168.…, 10.…, localhost), auch bei versehentlich offenem Port. Aus: auch von außen, etwa über Caddy — aber nur, wenn ein Mitarbeiter ein Passwort hat, und nur mit Anmeldung. Hinter Caddy zählt die Adresse des Besuchers; Caddy in Docker (172.…) braucht das Proxy-Token, dasselbe wie im Caddyfile.',
        'Home network only: only devices with a private address (192.168.…, 10.…, localhost), even with the port open by mistake. Off: from outside too, through Caddy say — but only once an employee has a password, and only after logging in. Behind Caddy the visitor\'s address counts; Caddy in Docker (172.…) needs the proxy token, the same as in the Caddyfile.')));
    }
    if (part === 'mail') children.push(renderMailboxes(cfg));
    if (part === 'mail') {
      children.push(el('p', 'muted', T(`Gespeichert in ${settings.path}, die vorige Fassung daneben als config.yaml.bak. Weitere Einstellungen beim jeweiligen Bereich: Kunden bei den Reisekosten, Harvest und sevDesk bei der Kundenrechnung, Anthropic, Apple und Vodafone bei den Belegen.`,
        `Saved in ${settings.path}, the version before beside it as config.yaml.bak. Further settings are with their section: clients with Travel, Harvest and sevDesk with Invoice, Anthropic, Apple and Vodafone with Receipts.`)));
    }
    form.replaceChildren(...children);
    form.onsubmit = (e) => e.preventDefault(); // saved with the button at the bottom
  }
}

// ── Saving: one button for everything, asked twice ─────────────────────────

/** What has changed and is not saved yet, by name. */
function unsaved() {
  const parts = [];
  if (settings && JSON.stringify(settings.config) !== settings.saved) parts.push('settings');
  if (schedule && scheduleJSON() !== schedule.saved) parts.push('schedule');
  if (lists.assignments.dirty) parts.push('assignments');
  if (lists.blocked.dirty) parts.push('blocked');
  if (logoDraft !== undefined) parts.push('logo');
  return parts;
}

/** The unsaved parts by their names on the page. */
const PART_NAMES = T({ settings: 'Einstellungen', schedule: 'Zeitplan', assignments: 'Zuordnungen', blocked: 'Gesperrte Tage', logo: 'Logo' },
  { settings: 'settings', schedule: 'schedule', assignments: 'assignments', blocked: 'blocked days', logo: 'logo' });
const partNames = (parts) => parts.map((p) => PART_NAMES[p]).join(', ');

let armed = null; // the timer while the button asks "sicher?"

function refreshSave() {
  const parts = unsaved();
  const save = $('save-all');
  save.disabled = parts.length === 0;
  $('discard-all').disabled = parts.length === 0;
  if (!parts.length) disarm();
  $('save-note').textContent = parts.length ? `${T('Nicht gespeichert', 'Not saved')}: ${partNames(parts)}` : '';
}

function disarm() {
  clearTimeout(armed);
  armed = null;
  $('save-all').classList.remove('armed');
  $('save-all').textContent = T('Speichern', 'Save');
}
disarm();
$('discard-all').textContent = T('Abbrechen', 'Discard');

// Abbrechen: what is not saved yet dropped, everything as saved again.
$('discard-all').addEventListener('click', async () => {
  const parts = unsaved();
  if (!parts.length) return;
  disarm();
  for (const name of Object.keys(lists)) {
    if (lists[name].dirty) {
      lists[name].dirty = false;
      stopEditing(name);
    }
  }
  logoDraft = undefined;
  logoConfirm = false;
  settings = null;
  schedule = null;
  await Promise.all([loadSettings(), loadCron(), loadTravel()]);
  refreshSave();
  $('save-note').textContent = `${T('Verworfen', 'Discarded')}: ${partNames(parts)}.`;
});

$('save-all').addEventListener('click', async () => {
  if (!armed) {
    $('save-all').classList.add('armed');
    $('save-all').textContent = T('Sicher?', 'Sure?');
    armed = setTimeout(disarm, 4000);
    return;
  }
  disarm();
  const parts = unsaved();
  $('save-all').disabled = true;
  $('save-note').textContent = T('Speichere…', 'Saving…');
  try {
    // The settings first: an assignment may name a client added just now.
    if (parts.includes('settings')) await api('PUT', '/api/settings', settings.config);
    if (parts.includes('schedule')) await api('PUT', '/api/schedule', JSON.parse(scheduleJSON()));
    if (parts.includes('logo')) {
      if (logoDraft) {
        const form = new FormData();
        form.append('logo', logoDraft);
        await api('PUT', '/api/logo', form);
      } else {
        await api('DELETE', '/api/logo');
      }
      logoDraft = undefined;
      logoStamp = Date.now();
    }
    for (const name of Object.keys(lists)) {
      if (lists[name].dirty) {
        await api('PUT', lists[name].path, lists[name].draft);
        lists[name].dirty = false;
        stopEditing(name);
      }
    }
  } catch (err) {
    $('save-note').textContent = `${T('Nicht gespeichert', 'Not saved')}: ${err.message}`;
    $('save-all').disabled = false;
    return;
  }
  settings = null;
  schedule = null;
  overview = null;
  await loadOverview();
  await Promise.all([loadSettings(), loadCron(), loadTravel()]);
  refreshSave();
  $('save-note').textContent = `${T('Gespeichert', 'Saved')}: ${partNames(parts)}.`;
});

// Every edit may change what is unsaved.
document.addEventListener('input', () => setTimeout(refreshSave));
document.addEventListener('change', () => setTimeout(refreshSave));

// ── Logo: chosen here, kept with the button at the bottom ────────────────────

let logoDraft; // undefined: unchanged; a File: to keep; null: to take away
let logoStamp = Date.now(); // a new one after saving, so that the browser fetches it again
let logoConfirm = false; // the × clicked: asking whether the logo should go

/** The company's logo: in the work log's head instead of its name. PNG or JPEG. */
function renderLogo() {
  // A file chosen or dropped: checked, and kept as the draft until Speichern.
  const take = (file) => {
    if (!file) return;
    if (!['image/png', 'image/jpeg'].includes(file.type)) {
      alert(T('Das Logo muss ein PNG oder JPEG sein.', 'The logo must be a PNG or JPEG.'));
      return;
    }
    if (file.size > 2 << 20) {
      alert(T('Das Logo ist größer als 2 MB.', 'The logo is larger than 2 MB.'));
      return;
    }
    logoDraft = file;
    logoConfirm = false;
    renderSettings();
    refreshSave();
  };
  const input = el('input');
  input.type = 'file';
  input.accept = 'image/png,image/jpeg';
  input.hidden = true;
  input.addEventListener('change', () => take(input.files[0]));

  const card = el('div', 'logo-card');
  card.addEventListener('dragover', (e) => { e.preventDefault(); card.classList.add('over'); });
  card.addEventListener('dragleave', () => card.classList.remove('over'));
  card.addEventListener('drop', (e) => {
    e.preventDefault();
    card.classList.remove('over');
    take(e.dataTransfer.files[0]);
  });

  // Empty: the button, and where a file may be dropped instead.
  const empty = el('div', 'logo-empty',
    button(T('Logo hochladen', 'Upload logo'), '', () => input.click()),
    el('span', 'muted', T('oder hier hineinziehen', 'or drop it here')),
    el('span', 'muted', T('.jpg, .jpeg, .png (max. 2 MB, 1 Datei)', '.jpg, .jpeg, .png (max. 2 MB, 1 file)')));

  // With a logo: a red × over it, and asked once more before it goes.
  const preview = el('img', 'logo-preview');
  preview.alt = 'Logo';
  const remove = button('×', 'logo-remove', () => { logoConfirm = true; renderSettings(); });
  const removeText = T('Firmenlogo löschen', 'Delete company logo');
  remove.title = removeText;
  remove.setAttribute('aria-label', removeText);
  const confirmBox = el('div', 'logo-confirm',
    button(removeText, 'danger', () => {
      logoDraft = null;
      logoConfirm = false;
      renderSettings();
      refreshSave();
    }),
    button(T('Abbrechen', 'Cancel'), '', () => { logoConfirm = false; renderSettings(); }));
  const filled = el('div', `logo-filled${logoConfirm ? ' confirming' : ''}`, preview, remove, confirmBox);
  confirmBox.hidden = !logoConfirm;
  remove.hidden = logoConfirm;

  const show = (src) => {
    empty.hidden = !!src;
    filled.hidden = !src;
    if (src) preview.src = src;
  };
  preview.onerror = () => show(null);
  if (logoDraft) show(URL.createObjectURL(logoDraft));
  else if (logoDraft === null) show(null);
  else show(`/api/logo?t=${logoStamp}`);

  card.append(empty, filled, input);
  return el('div', 'logo-box',
    el('p', 'muted', T('Oben rechts im Zeitnachweis, Größe und Lage wie auf den sevDesk-Rechnungen — am besten dieselbe Datei. Ohne Logo: der Firmenname, ohne den der Name des Harvest-Kontos.',
      "At the top right of the work log, size and place as on the sevDesk invoices — best the same file. Without a logo: the company name, without that the Harvest account's name.")),
    card);
}

/**
 * Other mailboxes the receipts come to — Anthropic's to another address, say — searched after
 * the main one, by IMAP only: the mail still goes out from the main one. A password left empty is
 * the one saved.
 */
function renderMailboxes(cfg) {
  cfg.Mail ??= {};
  cfg.Mail.Mailboxes ??= [];
  const boxes = cfg.Mail.Mailboxes;
  const changed = () => { renderSettings(); refreshSave(); };
  const list = el('ul', 'rows staff mailboxes', ...boxes.map((box, i) => {
    const user = el('input', 'grow');
    user.value = box.Username ?? '';
    user.placeholder = T('Adresse, z. B. max@example.de', 'Address, e.g. max@example.com');
    user.autocomplete = 'off';
    user.setAttribute('aria-label', T('Adresse', 'Address'));
    user.addEventListener('input', () => { box.Username = user.value.trim(); refreshSave(); });
    const password = el('input', 'password');
    password.type = 'password';
    password.autocomplete = 'new-password';
    password.placeholder = settings.mailboxes?.[i] ? T('gespeichert — neu eingeben', 'saved — type anew') : T('Passwort', 'Password');
    password.value = box.Password ?? '';
    password.setAttribute('aria-label', T('Passwort', 'Password'));
    password.addEventListener('input', () => { box.Password = password.value; refreshSave(); });
    const host = el('input', 'password');
    host.value = box.IMAPHost ?? '';
    host.placeholder = `IMAP: ${cfg.Mail.IMAPHost || T('wie oben', 'as above')}`;
    host.setAttribute('aria-label', T('IMAP-Server', 'IMAP server'));
    host.addEventListener('input', () => { box.IMAPHost = host.value.trim(); refreshSave(); });
    return el('li', '', user, password, host, button(T('entfernen', 'remove'), 'link', () => {
      boxes.splice(i, 1);
      settings.mailboxes?.splice(i, 1);
      changed();
    }));
  }));
  return el('div', '',
    el('h2', '', T('Weitere Postfächer', 'Other mailboxes')),
    el('p', 'muted', T('Weitere Postfächer mit Belegen. Durchsucht nach dem obigen, in denselben Ordnern; gesendet wird vom obigen. IMAP-Server leer: der obige.',
      'Other mailboxes with receipts. Searched after the one above, in the same folders; mail goes out from the one above. IMAP server empty: the one above.')),
    boxes.length ? list : el('p', 'muted', T('Keine.', 'None.')),
    el('div', 'actions', button(T('Postfach hinzufügen', 'Add mailbox'), '', () => { boxes.push({ Username: '' }); changed(); })));
}

/**
 * The employees, the selected one ticked: the Lieferant on the travel expenses, and whose hours
 * in Harvest make the work log and the invoice. Saved with the button at the bottom.
 */
function renderStaff(cfg) {
  cfg.Staff ??= {};
  cfg.Staff.Names ??= [];
  const staff = cfg.Staff;
  const changed = () => { renderSettings(); refreshSave(); };
  const list = el('ul', 'rows staff employees', ...staff.Names.map((name, i) => {
    const pick = el('input');
    pick.type = 'radio';
    pick.name = 'staff-current';
    pick.checked = name === staff.Current;
    pick.setAttribute('aria-label', T(`${name || 'Mitarbeiter'} auswählen`, `Select ${name || 'employee'}`));
    pick.addEventListener('change', () => { staff.Current = staff.Names[i]; changed(); });
    const input = el('input', 'grow');
    input.value = name;
    input.placeholder = T('Vorname Nachname, wie in Harvest', 'First and last name, as in Harvest');
    input.setAttribute('aria-label', 'Name');
    // Renaming the selected one keeps it selected; the list redraws once the field is left.
    input.addEventListener('input', () => {
      if (staff.Current === staff.Names[i]) staff.Current = input.value;
      staff.Names[i] = input.value;
      refreshSave();
    });
    input.addEventListener('change', changed);
    // The employee's password for logging in: typed anew, or none any more. Kept only as a hash.
    const has = settings.logins?.[name] && !(staff.NoLogin ?? []).includes(name);
    const password = el('input', 'password');
    password.type = 'password';
    password.autocomplete = 'new-password';
    password.placeholder = has ? T('gesetzt — neu eingeben', 'set — type anew') : T('Passwort für Anmeldung', 'password for login');
    password.value = staff.Passwords?.[name] ?? '';
    password.setAttribute('aria-label', T(`Passwort für ${name || 'den Mitarbeiter'}`, `Password for ${name || 'the employee'}`));
    password.addEventListener('input', () => {
      staff.Passwords ??= {};
      if (password.value) staff.Passwords[name] = password.value;
      else delete staff.Passwords[name];
      refreshSave();
    });
    // Its place kept empty without a password: the columns stay where they are.
    const noLogin = has ? button(T('Passwort entfernen', 'remove password'), 'link', () => {
      staff.NoLogin = [...(staff.NoLogin ?? []), name];
      changed();
    }) : el('span');
    return el('li', '', pick, input, password, noLogin, button(T('entfernen', 'remove'), 'link', () => {
      staff.Names.splice(i, 1);
      if (staff.Current === name) staff.Current = staff.Names[0] ?? '';
      changed();
    }));
  }));
  const add = button(T('Mitarbeiter hinzufügen', 'Add employee'), '', () => {
    staff.Names.push('');
    if (!staff.Current) staff.Current = '';
    renderSettings();
    refreshSave();
    const inputs = document.querySelectorAll('.staff input.grow');
    inputs[inputs.length - 1]?.focus();
  });
  return el('div', '',
    el('p', 'muted', T('Der ausgewählte Mitarbeiter ist Lieferant der Reisekosten; seine Harvest-Stunden ergeben Zeitnachweis und Kundenrechnung — Name wie in Harvest.',
      'The selected employee is the supplier on the travel expenses; their Harvest hours make the work log and the invoice — the name as in Harvest.')),
    el('p', 'muted', T('Passwort: mindestens 8 Zeichen, nur als Hash gespeichert. Hat einer eines, fragt die Seite nach der Anmeldung (im Heimnetz ohne Passwort); hat keiner eines, antwortet sie nur im Heimnetz.',
      'Password: at least 8 characters, kept only as a hash. Once anyone has one, the page asks for a login (at home without the password); while no one has, it answers the home network only.')),
    staff.Names.length ? list : el('p', 'muted', T('Noch niemand eingetragen.', 'No one entered yet.')),
    el('div', 'actions', add));
}

function renderClients(cfg) {
  cfg.TravelExpense.Clients ??= [];
  const box = el('div', '', el('p', 'muted',
    T('Feiertage: in Deutschland die des Bundeslands, sonst die des Landes. Kunden außerhalb Europas bekommen nur zugeordnete Tage. Im Ausland die Pauschalen des Landes eintragen: ',
      "Holidays: in Germany the state's, elsewhere the country's. Clients outside Europe get assigned days only. Abroad, enter the country's allowances: "),
    bmfLink(), T(' (ganzer Tag = 24 Stunden, An-/Abreisetag = An- und Abreisetag).', ' (full day = 24 hours, arrival/departure day = the day of arrival and of departure).')));
  cfg.TravelExpense.Clients.forEach((c, i) => {
    box.append(el('div', 'client',
      button(T('entfernen', 'remove'), 'link remove', () => { cfg.TravelExpense.Clients.splice(i, 1); renderSettings(); refreshSave(); }),
      el('div', 'grid', ...CLIENT_FIELDS
        .filter(([key]) => key !== 'Province' || (c.Country || 'DE').toUpperCase() === 'DE')
        .map((f) => field(c, f)))));
  });
  box.append(el('div', 'actions', button(T('Kunde hinzufügen', 'Add client'), '', () => {
    cfg.TravelExpense.Clients.push({ Province: 'BW' });
    renderSettings();
    refreshSave();
  })));
  return box;
}

// ── Start ──────────────────────────────────────────────────────────────────

loadOverview();
showTab();
setInterval(loadOverview, 10_000);
