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
  status.textContent = overview.running ? overview.running.title : nextText(overview.next);
  $('employee').hidden = !overview.employee;
  $('employee').textContent = `Mitarbeiter: ${overview.employee}`;
  for (const p of document.querySelectorAll('[data-last]')) p.textContent = lastText(overview.last[p.dataset.last]);
  for (const span of document.querySelectorAll('[data-mail-to]')) span.textContent = mailTo();
}

const SHORT = { 'travel-expense': 'Reisekosten', 'apple-invoice': 'Apple', 'vodafone-invoice': 'Vodafone', 'harvest-invoice': 'Kundenrechnung' };

/** "nächster Lauf: Vodafone · Sa., 21.11. 07:30", from the schedule. */
function nextText(next) {
  if (!next) return 'kein Lauf geplant';
  const at = new Date(next.at).toLocaleString('de-DE', { weekday: 'short', day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
  return `nächster Lauf: ${SHORT[next.job] || next.job} · ${at}`;
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
        button('entfernen', 'link', () => { schedule.entries.splice(i, 1); renderSchedule(); refreshSave(); })),
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

// The month the Harvest API is asked for, as on the command line: the current one, or an
// earlier one after the fact.
{ const now = new Date(); $('harvest-month').value = `${now.getMonth() + 1}/${now.getFullYear()}`; }
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

const KIND_TEXT = { weekend: '', holiday: 'Feiertag', christmas: 'Weihnachten', blocked: 'gesperrt', home: 'kein Kunde' };

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
};
const SHADES = { '#': 1, '+': 0.55, '.': 0.28 };

// The moon on a day away, 8 × 8 and solid, a marker in the corner: rectangles x y width height.
const MOON = [[2, 0, 4, 1], [1, 1, 3, 1], [0, 2, 3, 4], [1, 6, 3, 1], [2, 7, 4, 1], [6, 6, 1, 1]];

/** A pixel icon, its meaning as its title: meal, car and bed of GRIDS, or the moon. */
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

/** An amount after its icon: "🍴 44,00 €". */
const amount = (name, title, eur) => el('span', 'amount', icon(name, title), euro(eur));

function renderCalendar() {
  const cells = ['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So'].map((d) => el('div', 'wd', d));
  const first = (new Date(`${plan.month}-01T00:00:00`).getDay() + 6) % 7;
  for (let i = 0; i < first; i++) cells.push(el('div', 'day pad'));
  const today = new Date().toISOString().slice(0, 10);
  for (const d of plan.days) {
    const classes = ['day', d.kind, d.clientNo && colourOf(d.clientNo), d.assigned && 'assigned', d.date === today && 'today', (d.missing || d.lodgingMissing) && 'fallback'].filter(Boolean).join(' ');
    const who = d.client ? el('span', 'who', d.client) : el('span', 'note', d.note || KIND_TEXT[d.kind] || '');
    // What the day is worth, each amount after its icon: meals, the drive, the night.
    const eur = d.meal || d.mileage || d.lodging ? el('span', 'eur',
      d.meal && amount('meal', 'Verpflegung', d.meal),
      d.mileage && amount('car', 'Kilometergeld', d.mileage),
      d.lodging && amount('bed', 'Übernachtung', d.lodging)) : null;
    const cell = el('div', classes, el('span', 'n', String(Number(d.date.slice(8)))), d.kind === 'away' && el('span', 'moon', icon('moon', 'über Nacht')), who,
      d.client && d.note && el('span', 'note muted', d.note), eur);
    cell.title = [dmy(d.date), d.client, d.mealText && `Verpflegung ${d.mealText}`, d.lodging && `Übernachtung ${euro(d.lodging)}`, d.note,
      d.missing && `Satz „${RATE_NAMES[d.missing]}“ des Kunden fehlt — Inlandssatz ${euro(d.meal)}`,
      d.lodgingMissing && `Satz „${RATE_NAMES.lodging}“ des Kunden fehlt — Inlandssatz ${euro(d.lodging)}`].filter(Boolean).join(' · ');
    cells.push(cell);
  }
  $('calendar').replaceChildren(...cells);
  // The legend: each client in its colour.
  $('legend-clients').replaceChildren(...(plan.clients ?? []).map((name, i) => el('span', `k ${colourOf(i + 1)}`, name)));
  $('legend-icons').replaceChildren(
    el('span', '', icon('moon', 'über Nacht'), 'über Nacht'),
    el('span', '', icon('meal', 'Verpflegung'), 'Verpflegung'),
    el('span', '', icon('car', 'Kilometergeld'), 'Kilometergeld'),
    el('span', '', icon('bed', 'Übernachtung'), 'Übernachtung'));
}

/** A client's colour class, by its place in the list from 1: six colours, then again. */
const colourOf = (no) => `c${((no - 1) % 6) + 1}`;

const RATE_NAMES = { mealDay: 'Tagesreise > 8 h', mealFullDay: 'Ganzer Tag', mealTravelDay: 'An-/Abreisetag', lodging: 'Übernachtung' };

/** What the month's expenses come to: meals, mileage and nights. */
const totalOf = (t) => (t.meals ?? 0) + (t.mileage ?? 0) + (t.lodging ?? 0);

function renderTotals() {
  const t = plan.totals;
  // Clients abroad falling back to the inland rate: named once each, under the totals.
  const missing = new Map();
  for (const d of plan.days) {
    if (d.missing) missing.set(`${d.client}|${d.missing}`, `${d.client}: Satz „${RATE_NAMES[d.missing]}“ fehlt — es gilt der Inlandssatz ${euro(d.meal)}`);
    if (d.lodgingMissing) missing.set(`${d.client}|lodging`, `${d.client}: Satz „${RATE_NAMES.lodging}“ fehlt — es gilt der Inlandssatz ${euro(d.lodging)}`);
  }
  const per = Object.entries(plan.perClient).map(([name, n]) => `${name} ${n}`).join(' · ');
  $('travel-totals').replaceChildren(...[
    el('div', '', el('dt', '', 'Reisetage'), el('dd', '', String(t.days ?? 0))),
    el('div', '', el('dt', '', icon('meal', 'Verpflegung'), 'Verpflegung'), el('dd', '', euro(t.meals))),
    el('div', '', el('dt', '', icon('car', 'Kilometergeld'), 'Kilometer'), el('dd', '', euro(t.mileage))),
    t.nights > 0 && el('div', '', el('dt', '', icon('bed', 'Übernachtung'), 'Übernachtung'), el('dd', '', euro(t.lodging))),
    el('div', '', el('dt', '', 'Gesamt'), el('dd', '', euro(totalOf(t)))),
    per && el('div', 'whole', el('dt', '', 'Je Kunde'), el('dd', 'muted', per)),
    missing.size > 0 && el('div', 'whole warn', el('dt', '', 'Satz fehlt'),
      el('dd', '', ...[...missing.values()].flatMap((text, i) => (i ? [el('br'), text] : [text])), el('br'),
        'Beim Kunden eintragen: ', bmfLink())),
  ].filter(Boolean));
}

function renderRanges() {
  const clients = overview?.clients ?? [];
  const select = $('a-client');
  const chosen = select.value;
  // No client chosen to begin with: an assignment is made on purpose, not by default.
  const none = el('option', '', 'Kunde wählen…');
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
  assignments: { draft: null, editing: -1, dirty: false, path: '/api/travel/assignments', form: 'assignment-form', add: 'Zuordnen' },
  blocked: { draft: null, editing: -1, dirty: false, path: '/api/travel/blocked', form: 'blocked-form', add: 'Sperren' },
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
    assignments: (a) => [nameOf(a.client), a.overnight ? ' · über Nacht' : ' · Tagesreisen', a.overnight && a.hotel ? ' · Hotel mit Rechnung' : '', a.overnight && a.nightFlight ? ' · Rückflug über Nacht' : '', a.note ? ` · ${a.note}` : ''],
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
      button('bearbeiten', 'link', () => editEntry(name, i)),
      button('entfernen', 'link', () => {
        list.draft.splice(i, 1);
        if (list.editing === i) stopEditing(name);
        else if (list.editing > i) list.editing--;
        renderLists();
      }))));
    const form = $(list.form);
    form.querySelector('button[type=submit]').textContent = list.editing >= 0 ? 'Übernehmen' : list.add;
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
 * "Hotel mit Rechnung" and "Rückflug über Nacht" are for overnight trips only: their nights at
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
  if (!confirm(`Reisekosten ${MONTHS[month.getMonth()]} ${month.getFullYear()} senden an ${mailTo()}?\n${t.days ?? 0} Reisetage, ${euro(totalOf(t))}`)) return;
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

/**
 * The BMF's table of meal and overnight rates abroad, for the year of the month shown: the
 * server finds that year's in the Lohnsteuer-Handbuch, or the year before's while it is not out
 * yet — no link to move on each year.
 */
function bmfLink() {
  const year = plan?.month ? Number(plan.month.slice(0, 4)) : new Date().getFullYear();
  const a = el('a', '', 'BMF-Ländertabelle');
  a.href = `/api/bmf-table?year=${year}`;
  a.target = '_blank';
  a.rel = 'noopener';
  return a;
}

const PROVINCES = ['BW', 'BY', 'BE', 'BB', 'HB', 'HH', 'HE', 'MV', 'NI', 'NW', 'RP', 'SL', 'SN', 'ST', 'SH', 'TH'];
// A new line in a form's grid: what follows starts on the left.
const BREAK = ['', '', 'break'];

// The settings as the config file has them, each where it is used: per part of the page, its
// sections with field path, label, type. Only what all of them share is under Einstellungen.
const PARTS = {
  company: [['Firma', [['Company.Name', 'Firmenname']]]],
  staff: [['Mitarbeiter', []]],
  mail: [['E-Mail', [
    ['Mail.SMTPHost', 'SMTP-Server'], ['Mail.SMTPPort', 'SMTP-Port', 'number'],
    ['Mail.IMAPHost', 'IMAP-Server'], ['Mail.IMAPPort', 'IMAP-Port', 'number'],
    ['Mail.Username', 'Benutzer'], ['Mail.Password', 'Passwort', 'secret', 0],
    ['Mail.From', 'Absender'], ['Mail.To', 'Empfänger'], ['Mail.CC', 'Kopie (z. B. sevDesk-Autobox)'],
  ]]],
  web: [['Oberfläche', [
    ['Web.Listen', 'Adresse (Neustart nötig)', 'text', 'localhost:8595, auch im Netz (:8595)'],
    ['Web.Password', 'Passwort für diese Seite', 'secret', 3],
    ['Web.HomeOnly', 'Nur im Heimnetz erreichbar', 'bool', true],
  ]]],
  travel: [['Kunden', [
    ['TravelExpense.ChristmasWeekOff', 'Weihnachtswoche frei (24. und 27.–31.12.)', 'bool', true],
  ]], ['Pauschalen Inland', [
    ['TravelExpense.MealRates.Day', 'Tagesreise > 8 h (€)', 'money', 14],
    ['TravelExpense.MealRates.FullDay', 'Ganzer Tag (€)', 'money', 28],
    ['TravelExpense.MealRates.TravelDay', 'An-/Abreisetag (€)', 'money', 14],
    ['TravelExpense.LodgingRate', 'Übernachtung (€)', 'money', 20],
  ]]],
  invoice: [['', [
    ['HarvestInvoice.SevDesk.ClientName', 'Kunde in sevDesk'],
    ['HarvestInvoice.SevDesk.ProductName', 'Artikel (Suchbegriff)'], ['HarvestInvoice.SevDesk.ProductNum', 'Artikelnummer'],
    ['HarvestInvoice.SevDesk.ReferenceNum', 'Kundenreferenz (E-Rechnung)'],
    BREAK,
    ['HarvestInvoice.SevDesk.Username', 'sevDesk-Benutzer'], ['HarvestInvoice.SevDesk.Password', 'sevDesk-Passwort', 'secret', 2],
    BREAK,
    ['HarvestInvoice.Harvest.Token', 'Harvest-API-Token', 'secret', 4],
    ['HarvestInvoice.Harvest.Client', 'Kunde in Harvest', 'shadow', 'HarvestInvoice.SevDesk.ClientName'],
    BREAK,
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
  ['MealTravelDay', 'An-/Abreisetag (€)', 'money', 14], ['Lodging', 'Übernachtung (€)', 'money', 20],
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
        input.placeholder = (obj.Country || 'DE').toUpperCase() === 'DE' ? `leer: ${inland} (Inland)` : 'Satz des Landes';
      } else if (path.startsWith('Meal')) {
        // A client's own rate: else the inland one — abroad that is wrong, the country's is wanted.
        const inland = getPath(settings.config, `TravelExpense.MealRates.${path.slice(4)}`) || extra;
        // Abroad, the day trip and travel day rates are one in the BMF's table: each stands in
        // for the other.
        const other = { MealDay: 'MealTravelDay', MealTravelDay: 'MealDay' }[path];
        input.placeholder = (obj.Country || 'DE').toUpperCase() === 'DE' ? `leer: ${inland} (Inland)`
          : other && obj[other] ? `leer: ${obj[other]} (wie ${path === 'MealDay' ? 'An-/Abreisetag' : 'Tagesreise'})`
          : 'Satz des Landes';
      } else {
        input.placeholder = `Standard: ${extra} (gesetzlich)`;
      }
    }
    input.value = type === 'secret' ? '' : value || (type === 'number' || type === 'money' ? '' : value ?? '');
    if (type === 'secret') input.placeholder = settings.secretsSet[extra] ? 'gespeichert — zum Ändern eingeben' : '';
    // Left empty, the default applies: shown as the hint.
    if ((type === 'text' || type === 'number') && extra !== undefined) input.placeholder = `Standard: ${extra}`;
    // Left empty, another field's value applies: shown as the hint, as it is typed there.
    if (type === 'shadow') {
      input.dataset.shadowOf = extra;
      input.placeholder = getPath(obj, extra) ? `Standard: ${getPath(obj, extra)}` : '';
    }
    input.autocomplete = 'off';
  }
  bind(input, () => setPath(obj, path, type === 'number' || type === 'money' ? Number(input.value) || 0 : input.value));
  return el('label', '', label, input);
}

document.addEventListener('input', (e) => {
  const path = e.target.dataset && e.target.dataset.path;
  if (!path) return;
  for (const shadow of document.querySelectorAll(`[data-shadow-of="${path}"]`)) shadow.placeholder = e.target.value ? `Standard: ${e.target.value}` : '';
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
      if (title === 'Pauschalen Inland') {
        children.push(el('p', 'muted', 'Gilt für alle Kunden ohne eigene Sätze. Leer: die gesetzlichen Pauschalen (Verpflegung 14 / 28 / 14 €, Übernachtung ohne Hotelrechnung 20 €). Ändert sich das Gesetz, hier anpassen. Die Sätze fürs Ausland, Verpflegung und Übernachtung: ', bmfLink(), '.'));
      }
      if (title === 'Kunden') children.push(renderClients(cfg));
      if (title === 'Mitarbeiter') children.push(renderStaff(cfg));
      if (fields.length) children.push(el('fieldset', 'grid', ...fields.map((f) => field(cfg, f))));
      if (title === 'Firma') children.push(renderLogo());
    }
    if (part === 'web') {
      children.push(el('p', 'muted', 'Nur im Heimnetz: Die Oberfläche antwortet nur Geräten mit privater Adresse (192.168.…, 10.…, localhost), allen anderen nicht — auch wenn der Port versehentlich offen ist. Ausgeschaltet antwortet sie jedem, der sie erreicht. Passwort: HTTP Basic Auth, beliebiger Benutzername.'));
    }
    if (part === 'mail') {
      children.push(el('p', 'muted', `Gespeichert in ${settings.path}; die vorige Fassung liegt daneben als config.yaml.bak. Die übrigen Einstellungen stehen bei ihrem Bereich: Kunden bei den Reisekosten, Harvest und sevDesk bei der Kundenrechnung, Apple und Vodafone bei den Belegen, der Zeitplan unter Zeitplan.`));
    }
    form.replaceChildren(...children);
    form.onsubmit = (e) => e.preventDefault(); // saved with the button at the bottom
  }
}

// ── Saving: one button for everything, asked twice ─────────────────────────

/** What has changed and is not saved yet, by name. */
function unsaved() {
  const parts = [];
  if (settings && JSON.stringify(settings.config) !== settings.saved) parts.push('Einstellungen');
  if (schedule && scheduleJSON() !== schedule.saved) parts.push('Zeitplan');
  if (lists.assignments.dirty) parts.push('Zuordnungen');
  if (lists.blocked.dirty) parts.push('Gesperrte Tage');
  if (logoDraft !== undefined) parts.push('Logo');
  return parts;
}

let armed = null; // the timer while the button asks "sicher?"

function refreshSave() {
  const parts = unsaved();
  const save = $('save-all');
  save.disabled = parts.length === 0;
  if (!parts.length) disarm();
  $('save-note').textContent = parts.length ? `Nicht gespeichert: ${parts.join(', ')}` : '';
}

function disarm() {
  clearTimeout(armed);
  armed = null;
  $('save-all').classList.remove('armed');
  $('save-all').textContent = 'Speichern';
}

$('save-all').addEventListener('click', async () => {
  if (!armed) {
    $('save-all').classList.add('armed');
    $('save-all').textContent = 'Sicher?';
    armed = setTimeout(disarm, 4000);
    return;
  }
  disarm();
  const parts = unsaved();
  $('save-all').disabled = true;
  $('save-note').textContent = 'Speichere…';
  try {
    // The settings first: an assignment may name a client added just now.
    if (parts.includes('Einstellungen')) await api('PUT', '/api/settings', settings.config);
    if (parts.includes('Zeitplan')) await api('PUT', '/api/schedule', JSON.parse(scheduleJSON()));
    if (parts.includes('Logo')) {
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
    $('save-note').textContent = `Nicht gespeichert: ${err.message}`;
    $('save-all').disabled = false;
    return;
  }
  settings = null;
  schedule = null;
  overview = null;
  await loadOverview();
  await Promise.all([loadSettings(), loadCron(), loadTravel()]);
  refreshSave();
  $('save-note').textContent = `Gespeichert: ${parts.join(', ')}.`;
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
      alert('Das Logo muss ein PNG oder JPEG sein.');
      return;
    }
    if (file.size > 2 << 20) {
      alert('Das Logo ist größer als 2 MB.');
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
    button('Logo hochladen', '', () => input.click()),
    el('span', 'muted', 'oder hier hineinziehen'),
    el('span', 'muted', '.jpg, .jpeg, .png (max. 2 MB, 1 Datei)'));

  // With a logo: a red × over it, and asked once more before it goes.
  const preview = el('img', 'logo-preview');
  preview.alt = 'Logo';
  const remove = button('×', 'logo-remove', () => { logoConfirm = true; renderSettings(); });
  remove.title = 'Firmenlogo löschen';
  remove.setAttribute('aria-label', 'Firmenlogo löschen');
  const confirmBox = el('div', 'logo-confirm',
    button('Firmenlogo löschen', 'danger', () => {
      logoDraft = null;
      logoConfirm = false;
      renderSettings();
      refreshSave();
    }),
    button('Abbrechen', '', () => { logoConfirm = false; renderSettings(); }));
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
    el('p', 'muted', 'Das Logo steht oben rechts im Zeitnachweis, so groß und so platziert wie auf den Rechnungen aus sevDesk — am besten dieselbe Datei. Ohne Logo steht dort der Firmenname, ohne den der Name des Harvest-Kontos.'),
    card);
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
  const list = el('ul', 'rows staff', ...staff.Names.map((name, i) => {
    const pick = el('input');
    pick.type = 'radio';
    pick.name = 'staff-current';
    pick.checked = name === staff.Current;
    pick.setAttribute('aria-label', `${name || 'Mitarbeiter'} auswählen`);
    pick.addEventListener('change', () => { staff.Current = staff.Names[i]; changed(); });
    const input = el('input', 'grow');
    input.value = name;
    input.placeholder = 'Vorname Nachname, wie in Harvest';
    input.setAttribute('aria-label', 'Name');
    // Renaming the selected one keeps it selected; the list redraws once the field is left.
    input.addEventListener('input', () => {
      if (staff.Current === staff.Names[i]) staff.Current = input.value;
      staff.Names[i] = input.value;
      refreshSave();
    });
    input.addEventListener('change', changed);
    return el('li', '', pick, input, button('entfernen', 'link', () => {
      staff.Names.splice(i, 1);
      if (staff.Current === name) staff.Current = staff.Names[0] ?? '';
      changed();
    }));
  }));
  const add = button('Mitarbeiter hinzufügen', '', () => {
    staff.Names.push('');
    if (!staff.Current) staff.Current = '';
    renderSettings();
    refreshSave();
    const inputs = document.querySelectorAll('.staff input.grow');
    inputs[inputs.length - 1]?.focus();
  });
  return el('div', '',
    el('p', 'muted', 'Der ausgewählte Mitarbeiter steht als Lieferant auf den Reisekosten, und seine Stunden aus Harvest ergeben Zeitnachweis und Kundenrechnung — den Namen so schreiben wie in Harvest. Oben rechts steht, wer ausgewählt ist.'),
    staff.Names.length ? list : el('p', 'muted', 'Noch niemand eingetragen.'),
    el('div', 'actions', add));
}

function renderClients(cfg) {
  cfg.TravelExpense.Clients ??= [];
  const box = el('div', '', el('p', 'muted',
    'Die Kunden der Reisekosten. In Deutschland zählen die Feiertage des Bundeslands, sonst die des Landes. Kunden außerhalb Europas bekommen nur Tage über Zuordnungen, nie aus der gleichmäßigen Verteilung. Im Ausland die Verpflegungspauschalen des Landes eintragen: ',
    bmfLink(), ' (ganzer Tag = 24 Stunden, An-/Abreisetag = An- und Abreisetag).'));
  cfg.TravelExpense.Clients.forEach((c, i) => {
    box.append(el('div', 'client',
      button('entfernen', 'link remove', () => { cfg.TravelExpense.Clients.splice(i, 1); renderSettings(); refreshSave(); }),
      el('div', 'grid', ...CLIENT_FIELDS
        .filter(([key]) => key !== 'Province' || (c.Country || 'DE').toUpperCase() === 'DE')
        .map((f) => field(c, f)))));
  });
  box.append(el('div', 'actions', button('Kunde hinzufügen', '', () => {
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
