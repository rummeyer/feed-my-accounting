'use strict';

const $ = (id) => document.getElementById(id);

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
    headers: body && !isForm ? { 'Content-Type': 'application/json' } : {},
    body: isForm ? body : body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
}

const euro = (v) => `${(v ?? 0).toFixed(2).replace('.', ',')} €`;
const dmy = (iso) => (iso ? iso.split('-').reverse().join('.') : '');
const MONTHS = ['Januar', 'Februar', 'März', 'April', 'Mai', 'Juni', 'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember'];
const never = (iso) => !iso || iso.startsWith('0001');

function ago(iso) {
  if (never(iso)) return '';
  const s = Math.round((Date.now() - new Date(iso)) / 1000);
  if (s < 60) return 'gerade eben';
  if (s < 3600) return `vor ${Math.round(s / 60)} min`;
  if (s < 86400) return `vor ${Math.round(s / 3600)} h`;
  return new Date(iso).toLocaleString('de-DE', { dateStyle: 'short', timeStyle: 'short' });
}

// ── Theme, as on rummeyer.com: auto → light → dark ─────────────────────────

$('theme').addEventListener('click', () => {
  const root = document.documentElement;
  const next = { auto: 'light', light: 'dark', dark: 'auto' }[root.dataset.themePref || 'auto'];
  root.dataset.themePref = next;
  if (next === 'auto') delete root.dataset.theme;
  else root.dataset.theme = next;
  try { localStorage.setItem('theme', next); } catch { /* private window */ }
});

// ── Tabs ───────────────────────────────────────────────────────────────────

const TABS = ['cron', 'receipts', 'travel', 'invoice', 'log', 'settings'];
const loaders = {};

function showTab() {
  const name = TABS.includes(location.hash.slice(1)) ? location.hash.slice(1) : 'cron';
  for (const t of TABS) $(`tab-${t}`).hidden = t !== name;
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
    $('status').textContent = 'nicht erreichbar';
    return;
  }
  $('version').textContent = overview.version;
  const status = $('status');
  status.classList.toggle('busy', !!overview.running);
  status.textContent = overview.running ? overview.running.title : `bereit · ${overview.employee || ''}`;
  for (const p of document.querySelectorAll('[data-last]')) p.textContent = lastText(overview.last[p.dataset.last]);
  for (const span of document.querySelectorAll('[data-mail-to]')) span.textContent = mailTo();
}

/** Where the mails go: Einstellungen › E-Mail, its recipient and any copy. */
function mailTo() {
  const m = overview?.mail;
  if (!m?.to) return 'den Empfänger aus Einstellungen › E-Mail (noch keiner eingetragen)';
  return m.cc ? `${m.to} (Kopie ${m.cc})` : m.to;
}

function lastText(job) {
  if (!job) return '';
  const how = { ok: 'erledigt', failed: 'fehlgeschlagen', running: 'läuft' }[job.status] || job.status;
  return `${job.title} — ${how}, ${ago(job.ended || job.started)}${job.error ? `: ${job.error}` : ''}`;
}

// ── Cron ───────────────────────────────────────────────────────────────────

let schedule = null;

async function loadCron() {
  try {
    schedule = await api('GET', '/api/schedule');
  } catch (err) {
    $('schedule').replaceChildren(el('p', 'badge failed', err.message));
    return;
  }
  renderSchedule();
}
loaders.cron = loadCron;

const when = (iso) => new Date(iso).toLocaleString('de-DE', { weekday: 'short', day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });

function renderSchedule() {
  const rows = schedule.entries.map((e, i) => {
    const job = el('select');
    job.append(...Object.entries(schedule.jobs).map(([name, title]) => {
      const o = el('option', '', title);
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
    const next = e.error ? el('span', 'badge failed', e.error)
      : e.enabled ? el('span', '', 'nächste: ', e.next.map(when).join(' · ') || '—')
      : el('span', 'muted', 'aus — nächste wären ', e.next.slice(0, 1).map(when).join('') || '—');
    return el('div', `entry ${e.enabled ? '' : 'off'}`,
      el('div', 'entry-head',
        el('label', 'check', on, 'an'), job, cron,
        button('entfernen', 'link', () => { schedule.entries.splice(i, 1); renderSchedule(); })),
      el('div', 'entry-next small', next),
      el('div', 'entry-last small muted', e.last ? `zuletzt: ${lastText(e.last)}` : 'noch nicht über die Oberfläche gelaufen'));
  });
  const output = el('div', 'console');
  output.dataset.console = 'cron';
  output.hidden = true;
  $('schedule').replaceChildren(...rows, output);
}

$('schedule-add').addEventListener('click', () => {
  schedule.entries.push({ job: 'travel-expense', cron: '0 6 5 * *', enabled: false, next: [] });
  renderSchedule();
});

$('schedule-save').addEventListener('click', async () => {
  try {
    await api('PUT', '/api/schedule', schedule.entries.map(({ job, cron, enabled }) => ({ job, cron, enabled })));
    $('schedule-note').textContent = 'Gespeichert.';
    loadCron();
  } catch (err) {
    $('schedule-note').textContent = err.message;
  }
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

function renderJob(box, job) {
  const pre = el('pre', '', (job.output ?? []).join('\n') || '…');
  const files = (job.files ?? []).map((f) => {
    const a = el('a', '', `${f.name} (${Math.round(f.size / 1024)} KB)`);
    a.href = `/api/jobs/${job.id}/files/${encodeURIComponent(f.name)}`;
    a.target = '_blank';
    return a;
  });
  box.replaceChildren(...[
    el('div', 'head', el('span', 'title', job.title), el('span', `badge ${job.status}`,
      { running: 'läuft', ok: 'erledigt', failed: 'fehlgeschlagen' }[job.status] || job.status)),
    pre,
    job.error && el('p', 'badge failed', job.error),
    files.length > 0 && el('div', 'files', ...files),
  ].filter(Boolean));
  pre.scrollTop = pre.scrollHeight;
}

async function start(name, method, path, body, onDone) {
  try {
    const job = await api(method, path, body);
    loadOverview();
    follow(name, job, onDone);
  } catch (err) {
    alert(err.message);
  }
}

for (const b of document.querySelectorAll('[data-run]')) {
  b.addEventListener('click', () => start(b.dataset.run, 'POST', `/api/${b.dataset.run}/run`));
}

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
  renderCalendar();
  renderTotals();
  renderRanges();
}
loaders.travel = () => { loadTravel(); loadSettings(); };

const KIND_TEXT = { weekend: '', holiday: 'Feiertag', christmas: 'Weihnachten', blocked: 'gesperrt', home: 'kein Kunde' };

function renderCalendar() {
  const cells = ['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So'].map((d) => el('div', 'wd', d));
  const first = (new Date(`${plan.month}-01T00:00:00`).getDay() + 6) % 7;
  for (let i = 0; i < first; i++) cells.push(el('div', 'day pad'));
  const today = new Date().toISOString().slice(0, 10);
  for (const d of plan.days) {
    const classes = ['day', d.kind, d.assigned && 'assigned', d.date === today && 'today'].filter(Boolean).join(' ');
    const who = d.client ? el('span', 'who', d.client) : el('span', 'note', d.note || KIND_TEXT[d.kind] || '');
    const eur = d.meal || d.mileage
      ? el('span', 'eur', `${euro(d.meal)}${d.mileage ? ` · ${euro(d.mileage)} km` : ''}`) : null;
    const cell = el('div', classes, el('span', 'n', String(Number(d.date.slice(8)))), who,
      d.client && d.note && el('span', 'note muted', d.note), eur);
    cell.title = [dmy(d.date), d.client, d.mealText && `Verpflegung ${d.mealText}`, d.note].filter(Boolean).join(' · ');
    cells.push(cell);
  }
  $('calendar').replaceChildren(...cells);
}

function renderTotals() {
  const t = plan.totals;
  const per = Object.entries(plan.perClient).map(([name, n]) => `${name} ${n}`).join(' · ');
  $('travel-totals').replaceChildren(...[
    el('div', '', el('dt', '', 'Reisetage'), el('dd', '', String(t.days ?? 0))),
    el('div', '', el('dt', '', 'Verpflegung'), el('dd', '', euro(t.meals))),
    el('div', '', el('dt', '', 'Kilometer'), el('dd', '', euro(t.mileage))),
    el('div', '', el('dt', '', 'Gesamt'), el('dd', '', euro((t.meals ?? 0) + (t.mileage ?? 0)))),
    per && el('div', 'whole', el('dt', '', 'Je Kunde'), el('dd', 'muted', per)),
  ].filter(Boolean));
}

function renderRanges() {
  const clients = overview?.clients ?? [];
  const select = $('a-client');
  const chosen = select.value;
  select.replaceChildren(...clients.map((c) => {
    const o = el('option', '', c.name);
    o.value = c.id || c.name;
    return o;
  }));
  if (chosen) select.value = chosen;
  const nameOf = (key) => clients.find((c) => c.id === key || c.name === key)?.name ?? key;
  const span = (r) => (r.to && r.to !== r.from ? `${dmy(r.from)} – ${dmy(r.to)}` : dmy(r.from));
  $('assignments').replaceChildren(...plan.assignments.map((a, i) => el('li', '',
    el('span', 'when', span(a)),
    el('span', 'grow', nameOf(a.client), a.overnight ? ' · über Nacht' : ' · Tagesreisen', a.note ? ` · ${a.note}` : ''),
    button('entfernen', 'link', () => saveAssignments(plan.assignments.filter((_, j) => j !== i))))));
  $('blocked').replaceChildren(...plan.blocked.map((b, i) => el('li', '',
    el('span', 'when', span(b)), el('span', 'grow', b.note || ''),
    button('entfernen', 'link', () => saveBlocked(plan.blocked.filter((_, j) => j !== i))))));
}

async function saveAssignments(list) {
  try {
    await api('PUT', '/api/travel/assignments', list);
    loadTravel();
  } catch (err) {
    alert(err.message);
  }
}

async function saveBlocked(list) {
  try {
    await api('PUT', '/api/travel/blocked', list);
    loadTravel();
  } catch (err) {
    alert(err.message);
  }
}

$('assignment-form').addEventListener('submit', (e) => {
  e.preventDefault();
  saveAssignments([...plan.assignments, {
    client: $('a-client').value, from: $('a-from').value, to: $('a-to').value,
    overnight: $('a-overnight').checked, note: $('a-note').value.trim(),
  }]);
  e.target.reset();
});

$('blocked-form').addEventListener('submit', (e) => {
  e.preventDefault();
  saveBlocked([...plan.blocked, { from: $('b-from').value, to: $('b-to').value, note: $('b-note').value.trim() }]);
  e.target.reset();
});

$('travel-generate').addEventListener('click', () =>
  start('travel', 'POST', '/api/travel/generate', { month: monthKey() }));
$('travel-send').addEventListener('click', () => {
  const t = plan?.totals ?? {};
  if (!confirm(`Reisekosten ${MONTHS[month.getMonth()]} ${month.getFullYear()} senden an ${mailTo()}?\n${t.days ?? 0} Reisetage, ${euro((t.meals ?? 0) + (t.mileage ?? 0))}`)) return;
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
  $('drop-text').textContent = `${file.name} wird gelesen…`;
  const form = new FormData();
  form.append('report', file);
  try {
    upload = await api('POST', '/api/harvest/upload', form);
  } catch (err) {
    $('drop-text').textContent = err.message;
    return;
  }
  $('drop-text').textContent = `${file.name}: ${String(upload.hours).replace('.', ',')} Stunden gelesen`;
  $('d-hours').value = upload.hours;
  const now = new Date();
  $('d-from').value = upload.from || `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-01`;
  $('d-to').value = upload.to || new Date(now.getFullYear(), now.getMonth() + 1, 0, 12).toISOString().slice(0, 10);
  $('d-client').value = upload.client || '— in den Einstellungen setzen —';
  $('d-product').value = upload.product;
  $('d-reference').value = upload.reference;
  $('draft-form').hidden = false;
}

$('draft-form').addEventListener('submit', (e) => {
  e.preventDefault();
  const hours = Number($('d-hours').value);
  if (!confirm(`Rechnungsentwurf in sevDesk anlegen?\n${upload.client}: ${String(hours).replace('.', ',')} Stunden, ${dmy($('d-from').value)} – ${dmy($('d-to').value)}`)) return;
  start('invoice', 'POST', '/api/harvest/draft', { id: upload.id, hours, from: $('d-from').value, to: $('d-to').value });
});

// ── Log ────────────────────────────────────────────────────────────────────

async function loadLog() {
  const jobs = await api('GET', '/api/jobs').catch(() => []);
  $('no-jobs').hidden = jobs.length > 0;
  $('jobs').replaceChildren(...jobs.map((job) => {
    const li = el('li', '',
      el('span', 'when', new Date(job.started).toLocaleString('de-DE', { dateStyle: 'short', timeStyle: 'short' })),
      el('span', 'grow title', job.title),
      el('span', `badge ${job.status}`, { running: 'läuft', ok: 'erledigt', failed: 'fehlgeschlagen' }[job.status] || job.status));
    li.addEventListener('click', () => {
      for (const other of $('jobs').children) other.classList.remove('on');
      li.classList.add('on');
      follow('log', job, () => {});
    });
    return li;
  }));
}
loaders.log = loadLog;

// ── Settings ───────────────────────────────────────────────────────────────

// The BMF's table of meal and overnight rates abroad, for the current year: a new one comes each
// December, then this link moves on.
const BMF_TABLE = {
  year: 2026,
  url: 'https://www.bundesfinanzministerium.de/Content/DE/Downloads/BMF_Schreiben/Steuerarten/Lohnsteuer/2025-12-05-steuerliche-behandlung-reisekosten-2026.html',
};

function bmfLink() {
  const a = el('a', '', `BMF-Ländertabelle ${BMF_TABLE.year}`);
  a.href = BMF_TABLE.url;
  a.target = '_blank';
  a.rel = 'noopener';
  return a;
}

const PROVINCES = ['BW', 'BY', 'BE', 'BB', 'HB', 'HH', 'HE', 'MV', 'NI', 'NW', 'RP', 'SL', 'SN', 'ST', 'SH', 'TH'];
// The settings as the config file has them, each where it is used: per part of the page, its
// sections with field path, label, type. Only what all of them share is under Einstellungen.
const PARTS = {
  mail: [['E-Mail', [
    ['Mail.SMTPHost', 'SMTP-Server'], ['Mail.SMTPPort', 'SMTP-Port', 'number'],
    ['Mail.IMAPHost', 'IMAP-Server'], ['Mail.IMAPPort', 'IMAP-Port', 'number'],
    ['Mail.Username', 'Benutzer'], ['Mail.Password', 'Passwort', 'secret', 0],
    ['Mail.From', 'Absender'], ['Mail.To', 'Empfänger'], ['Mail.CC', 'Kopie (z. B. sevDesk-Autobox)'],
  ]]],
  web: [['Oberfläche', [
    ['Web.Access', 'Erreichbar', 'select', [['home', 'Nur im Heimnetz'], ['proxy', 'Nur über Caddy (HTTPS, Passwort nötig)']]],
    ['Web.Password', 'Passwort für diese Seite', 'secret', 4],
    ['Web.Listen', 'Adresse (Neustart nötig)', 'text', 'localhost:8595, auch im Netz (:8595)'],
    ['Web.Proxies', 'Proxy-Adressen (bei „nur über Caddy“)', 'list', '127.0.0.1, ::1, 172.16.0.0/12'],
  ]]],
  travel: [['Kunden', [
    ['TravelExpense.Employee', 'Name (Lieferant im PDF)'],
    ['TravelExpense.ChristmasWeekOff', 'Weihnachtswoche frei (24. und 27.–31.12.)', 'bool', true],
  ]], ['Verpflegung Inland', [
    ['TravelExpense.MealRates.Day', 'Tagesreise > 8 h (€)', 'money', 14],
    ['TravelExpense.MealRates.FullDay', 'Ganzer Tag (€)', 'money', 28],
    ['TravelExpense.MealRates.TravelDay', 'An-/Abreisetag (€)', 'money', 14],
  ]]],
  invoice: [['Einstellungen', [
    ['HarvestInvoice.SevDesk.ClientName', 'Kunde in sevDesk'],
    ['HarvestInvoice.SevDesk.ProductName', 'Artikel (Suchbegriff)'], ['HarvestInvoice.SevDesk.ProductNum', 'Artikelnummer'],
    ['HarvestInvoice.SevDesk.ReferenceNum', 'Kundenreferenz (E-Rechnung)'],
    ['HarvestInvoice.SevDesk.Username', 'sevDesk-Benutzer'], ['HarvestInvoice.SevDesk.Password', 'sevDesk-Passwort', 'secret', 3],
    ['HarvestInvoice.Harvest.Username', 'Harvest-Benutzer'], ['HarvestInvoice.Harvest.Password', 'Harvest-Passwort', 'secret', 2],
    ['HarvestInvoice.Filter.Count', 'Mails durchsuchen', 'number', 10],
    ['HarvestInvoice.Filter.From', 'Absender-Domain', 'text', 'harvestapp.com'],
    ['HarvestInvoice.Filter.Subject', 'Betreff', 'text', 'We’ve exported your detailed time report'],
    ['HarvestInvoice.CurrentMonthOnly', 'Aus der Mail nur Berichte des laufenden Monats', 'bool', true],
    ['HarvestInvoice.SkipExisting', 'Nicht doppelt anlegen (sevDesk prüfen)', 'bool', true],
  ]]],
  apple: [['', [
    ['AppleInvoice.Filter.Count', 'Mails durchsuchen', 'number', 10],
    ['AppleInvoice.Filter.From', 'Absender-Domain', 'text', 'apple.com'],
    ['AppleInvoice.Filter.Subject', 'Betreff', 'text', 'Deine Rechnung von Apple'],
  ]]],
  vodafone: [['', [
    ['VodafoneInvoice.Username', 'Benutzer'], ['VodafoneInvoice.Password', 'Passwort', 'secret', 1],
    ['VodafoneInvoice.FallbackToLastMonth', 'Sonst die Rechnung vom Vormonat', 'bool', true],
  ]]],
};
const CLIENT_FIELDS = [
  ['ID', 'Nr.'], ['Name', 'Name'], ['From', 'Von'], ['To', 'Nach'], ['Reason', 'Grund'],
  ['Distance', 'Entfernung (km, einfach)', 'number'], ['Country', 'Land', 'country'], ['Province', 'Bundesland', 'province'],
  ['MealDay', 'Tagesreise > 8 h (€)', 'money', 14], ['MealFullDay', 'Ganzer Tag (€)', 'money', 28],
  ['MealTravelDay', 'An-/Abreisetag (€)', 'money', 14],
  ['OnlyAssigned', 'Nur zugeordnete Tage', 'bool', false],
];

let settings = null;

const getPath = (obj, path) => path.split('.').reduce((o, k) => o?.[k], obj);
function setPath(obj, path, value) {
  const keys = path.split('.');
  const last = keys.pop();
  keys.reduce((o, k) => (o[k] ??= {}), obj)[last] = value;
}

function field(obj, [path, label, type, extra]) {
  const value = getPath(obj, path);
  let input;
  if (type === 'bool') {
    input = el('input');
    input.type = 'checkbox';
    input.checked = value ?? extra;
    bind(input, () => setPath(obj, path, input.checked));
    return el('label', 'check span', input, label);
  }
  if (type === 'select') {
    input = el('select');
    input.append(...extra.map(([v, text]) => {
      const o = el('option', '', text);
      o.value = v;
      return o;
    }));
    input.value = value || extra[0][0];
    bind(input, () => setPath(obj, path, input.value));
    return el('label', '', label, input);
  }
  if (type === 'list') {
    input = el('input');
    input.value = (value ?? []).join(', ');
    input.placeholder = `Standard: ${extra}`;
    input.autocomplete = 'off';
    bind(input, () => setPath(obj, path, input.value.split(',').map((v) => v.trim()).filter(Boolean)));
    return el('label', '', label, input);
  }
  if (type === 'province') {
    input = el('select');
    input.append(...PROVINCES.map((p) => el('option', '', p)));
    input.value = value || 'BW';
  } else if (type === 'country') {
    input = el('select');
    const names = Object.entries(settings.countries).sort((a, b) => a[1].localeCompare(b[1], 'de'));
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
    if (type === 'money') {
      input.step = '0.01';
      if (path.startsWith('Meal')) {
        // A client's own rate: else the inland one — abroad that is wrong, the country's is wanted.
        const inland = getPath(settings.config, `TravelExpense.MealRates.${path.slice(4)}`) || extra;
        input.placeholder = (obj.Country || 'DE').toUpperCase() === 'DE' ? `leer: ${inland} (Inland)` : 'Satz des Landes';
      } else {
        input.placeholder = `Standard: ${extra} (gesetzlich)`;
      }
    }
    input.value = type === 'secret' ? '' : value || (type === 'number' || type === 'money' ? '' : value ?? '');
    if (type === 'secret') input.placeholder = settings.secretsSet[extra] ? 'gespeichert — zum Ändern eingeben' : '';
    // Left empty, the default applies: shown as the hint.
    if ((type === 'text' || type === 'number') && extra !== undefined) input.placeholder = `Standard: ${extra}`;
    input.autocomplete = 'off';
  }
  bind(input, () => setPath(obj, path, type === 'number' || type === 'money' ? Number(input.value) || 0 : input.value));
  return el('label', '', label, input);
}

function bind(input, update) {
  input.addEventListener('input', update);
  input.addEventListener('change', update);
  return input;
}

async function loadSettings() {
  try {
    settings = await api('GET', '/api/settings');
    settings.access = settings.config.Web?.Access || 'home';
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
      if (title === 'Verpflegung Inland') {
        children.push(el('p', 'muted', 'Gilt für alle Kunden ohne eigene Sätze. Leer: die gesetzlichen Pauschalen (14 / 28 / 14 €). Ändert sich das Gesetz, hier anpassen. Die Sätze fürs Ausland: ', bmfLink(), '.'));
      }
      if (title === 'Kunden') children.push(renderClients(cfg));
      children.push(el('fieldset', 'grid', ...fields.map((f) => field(cfg, f))));
    }
    const note = el('span', 'muted');
    children.push(el('div', 'actions', el('button', 'strong', 'Speichern'), note));
    if (part === 'web') {
      children.push(el('p', 'muted', 'Nur im Heimnetz: Die Seite antwortet nur Geräten mit privater Adresse (192.168.…, 10.…, localhost). Nur über Caddy: Sie antwortet nur Anfragen, die Caddy per HTTPS weiterreicht, und nur mit Passwort — direkt aufgerufen nie, auch nicht im Heimnetz. Passwort: HTTP Basic Auth, beliebiger Benutzername.'));
    }
    if (part === 'mail') {
      children.push(el('p', 'muted', `Gespeichert in ${settings.path}; die vorige Fassung liegt daneben als config.yaml.bak. Die übrigen Einstellungen stehen bei ihrem Bereich: Kunden bei den Reisekosten, Harvest und sevDesk bei der Kundenrechnung, Apple und Vodafone bei den Belegen, der Zeitplan unter Cron.`));
    }
    form.replaceChildren(...children);
    form.onsubmit = (e) => saveSettings(e, note);
  }
}

async function saveSettings(e, note) {
  e.preventDefault();
  // Through the proxy only, this page is gone directly: say so before it is.
  if (settings.config.Web?.Access === 'proxy' && settings.access !== 'proxy') {
    if (!confirm('Danach ist die Oberfläche nur noch über Caddy erreichbar: mit HTTPS und Passwort, nicht mehr direkt — auch diese Seite hier nicht. Zurück nur, indem in config.yaml „access: home“ eingetragen wird.\n\nSo speichern?')) return;
  }
  try {
    await api('PUT', '/api/settings', settings.config);
    overview = null;
    await loadSettings();
    await loadOverview();
    if (!$('tab-travel').hidden) loadTravel();
    note.textContent = '';
    for (const n of document.querySelectorAll('form.part .actions .muted')) n.textContent = '';
    e.target.querySelector('.actions .muted').textContent = 'Gespeichert.';
  } catch (err) {
    note.textContent = err.message;
  }
}

function renderClients(cfg) {
  cfg.TravelExpense.Clients ??= [];
  const box = el('div', '', el('p', 'muted',
    'Die Kunden der Reisekosten. In Deutschland zählen die Feiertage des Bundeslands, sonst die des Landes. Im Ausland die Verpflegungspauschalen des Landes eintragen: ',
    bmfLink(), ' (ganzer Tag = 24 Stunden, An-/Abreisetag = An- und Abreisetag).'));
  cfg.TravelExpense.Clients.forEach((c, i) => {
    box.append(el('div', 'client',
      button('entfernen', 'link remove', () => { cfg.TravelExpense.Clients.splice(i, 1); renderSettings(); }),
      el('div', 'grid', ...CLIENT_FIELDS
        .filter(([key]) => key !== 'Province' || (c.Country || 'DE').toUpperCase() === 'DE')
        .map((f) => field(c, f)))));
  });
  box.append(el('div', 'actions', button('Kunde hinzufügen', '', () => {
    cfg.TravelExpense.Clients.push({ Province: 'BW' });
    renderSettings();
  })));
  return box;
}

// ── Start ──────────────────────────────────────────────────────────────────

loadOverview();
showTab();
setInterval(loadOverview, 10_000);
