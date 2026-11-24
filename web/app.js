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
  if (!res.ok) throw Object.assign(new Error(data.error || `HTTP ${res.status}`), { data });
  return data;
}

// ── The page's own question and notice, in its look rather than the browser's ──

/** Asks text, the answer yes or no: resolves true on yes, false on no or Escape. */
function ask(text, { yes = T('OK', 'OK'), no = T('Abbrechen', 'Cancel') } = {}) {
  const dialog = $('ask');
  $('ask-text').textContent = text;
  $('ask-yes').textContent = yes;
  $('ask-no').textContent = no ?? '';
  $('ask-no').hidden = no === null;
  return new Promise((resolve) => {
    const done = (answer) => {
      dialog.removeEventListener('cancel', escape);
      $('ask-yes').onclick = $('ask-no').onclick = null;
      dialog.close();
      resolve(answer);
    };
    const escape = (e) => { e.preventDefault(); done(false); };
    dialog.addEventListener('cancel', escape);
    $('ask-yes').onclick = () => done(true);
    $('ask-no').onclick = () => done(false);
    dialog.showModal();
    $('ask-yes').focus();
  });
}

/** Tells text, with OK only. */
const tell = (text) => ask(text, { no: null });

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
/** Hours as the page writes them: 41,25 in German, 41.25 in English. */
const hoursText = (h) => T(String(h).replace('.', ','), String(h));

/** A drop target for a file: dragged over, it lights up; dropped, take gets the file. */
function dropZone(target, take) {
  target.addEventListener('dragover', (e) => { e.preventDefault(); target.classList.add('over'); });
  target.addEventListener('dragleave', () => target.classList.remove('over'));
  target.addEventListener('drop', (e) => {
    e.preventDefault();
    target.classList.remove('over');
    if (e.dataTransfer.files[0]) take(e.dataTransfer.files[0]);
  });
}
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

const TABS = ['closing', 'cron', 'travel', 'receipts', 'invoice', 'log', 'settings'];
const loaders = {};

function showTab() {
  // The closing first: the overview.
  const name = TABS.includes(location.hash.slice(1)) ? location.hash.slice(1) : 'closing';
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
  // Once a month only: the send button off while a login by hand would come too soon — after the
  // last run, or before the schedule's next, which it would make skip — and beside it, when.
  const day = (t) => new Date(t).toLocaleDateString(LOCALE, { day: '2-digit', month: '2-digit', year: 'numeric' });
  for (const note of document.querySelectorAll('[data-gate]')) {
    const gate = overview.gates?.[note.dataset.gate] ?? {};
    const button = document.querySelector(`[data-send="${note.dataset.gate}"]`);
    if (button) button.disabled = !!gate.closed;
    note.textContent = !gate.closed ? ''
      : gate.scheduled ? T(`per Zeitplan am ${day(gate.scheduled)}`, `by schedule on ${day(gate.scheduled)}`)
        : gate.next ? T(`manuell ab ${day(gate.next)} möglich`, `by hand from ${day(gate.next)}`) : '';
    note.title = gate.closed ? T('Nur eine Anmeldung in 28 Tagen: mehr sperrt Vodafone.', 'One login in 28 days only: more and Vodafone blocks it.') : '';
  }

  for (const span of document.querySelectorAll('[data-mail-to]')) span.textContent = mailTo();
}

const SHORT = { 'travel-expense': T('Reisekosten', 'Travel'), 'amazon-invoice': 'Amazon', 'anthropic-invoice': 'Anthropic', 'apple-invoice': 'Apple', 'vodafone-invoice': 'Vodafone', 'harvest-invoice': T('Kundenrechnung', 'Invoice'), 'mail-receipts': T('Weitere Absender', 'Further senders') };

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
  [/^Amazon-Rechnungen abholen/, 'Fetch Amazon invoices'],
  [/^Anthropic-Belege abholen/, 'Fetch Anthropic receipts'],
  [/^Apple-Rechnungen abholen/, 'Fetch Apple invoices'],
  [/^Vodafone-Rechnungen abholen/, 'Fetch Vodafone invoices'],
  [/^Weitere Absender abholen/, 'Fetch further senders'],
  [/^Weitere Absender (\S+) ansehen/, 'Further senders $1, view'],
  [/^Weitere Absender (\S+) senden/, 'Further senders $1, send'],
  [/^Kundenrechnung aus Harvest \(laufender Monat\)/, 'Invoice from Harvest (current month)'],
  [/^Reisekosten (\S+) erzeugen/, 'Travel expenses $1, view'],
  [/^Reisekosten (\S+) senden/, 'Travel expenses $1, send'],
  [/^Anthropic-Belege (\S+) ansehen/, 'Anthropic receipts $1, view'],
  [/^Anthropic-Belege (\S+) senden/, 'Anthropic receipts $1, send'],
  [/^(Amazon|Apple|Vodafone)-Rechnungen (\S+) ansehen/, '$1 invoices $2, view'],
  [/^(Amazon|Apple|Vodafone)-Rechnungen (\S+) senden/, '$1 invoices $2, send'],
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
    // By when each runs next, as loaded — one added stays where it is put until then. None
    // coming last.
    const next = (e) => (e.next?.length ? Date.parse(e.next[0]) : Infinity);
    schedule.entries.sort((a, b) => next(a) - next(b) || a.job.localeCompare(b.job));
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

// ── Live view: a page a run hands over — Vodafone's login with a captcha ────

// Asked or not, every two seconds: a scheduled run may ask too, with this page open. While it
// asks, the picture anew a little more often than once a second.
let liveTimer = null;
/** Why a run hands its page over, by the key it sends. */
const LIVE_REASONS = () => ({
  'no-password': T('Vodafone: kein Passwort gespeichert – bitte hier anmelden.', 'Vodafone: no password stored – please log in here.'),
  'no-form': T('Vodafone zeigt kein Login-Formular – bitte hier anmelden.', 'Vodafone shows no login form – please log in here.'),
  asks: T('Vodafone fragt nach – bitte hier beantworten.', 'Vodafone is asking something – please answer it here.'),
  refused: T('Vodafone lehnt die Anmeldung ab – bitte hier selbst anmelden, mit „Eingeloggt bleiben“; danach bleibt der Login für die nächsten Läufe.', 'Vodafone refuses the login – please log in here yourself, with “Eingeloggt bleiben”; the login then stays for the next runs.'),
  'amazon-password': T('Amazon will das Passwort: unten „Passwort“ drücken (das gespeicherte) oder eintippen und „Eintragen“.', 'Amazon wants the password: press “Password” below (the stored one) or type it and “Enter text”.'),
  'amazon-otp': T('Amazon will den Code aus der Authenticator-App: unten eintippen und „Eintragen“ – er wird gesendet. „Kein Code erforderlich“ ist schon angehakt.', 'Amazon wants the code from the authenticator app: type it below and “Enter text” – it is sent. “No code required” is ticked already.'),
  'amazon-captcha': T('Amazon will ein Rätsel oder die Zeichen aus dem Bild gelöst haben – bitte im Bild lösen.', 'Amazon wants a puzzle or the characters from the picture solved – please solve it in the picture.'),
  'amazon-refused': T('Amazon lehnt die Anmeldung ab – bitte im Bild selbst anmelden.', 'Amazon refuses the login – please log in yourself in the picture.'),
  'amazon-no-password': T('Amazon: kein Passwort gespeichert – bitte im Bild anmelden.', 'Amazon: no password stored – please log in in the picture.'),
  'amazon-asks': T('Amazon fragt nach – bitte im Bild beantworten.', 'Amazon is asking something – please answer it in the picture.'),
  captcha: T('Vodafone will einen Sicherheitscode – bitte den Code aus dem Bild ins Feld eintippen und „Weiter“ klicken.', 'Vodafone wants a security code – please type the code from the picture into the field and click “Weiter”.'),
});
async function checkLive() {
  const state = await api('GET', '/api/live').catch(() => null);
  const dialog = $('live');
  if (state?.asking) {
    $('live-reason').textContent = LIVE_REASONS()[state.reason] ?? state.reason;
    $('live-fill').hidden = !state.fill;
    if (!dialog.open) {
      dialog.showModal();
      liveReset();
      refreshShot();
      liveTimer = setInterval(refreshShot, 800);
    }
  } else if (dialog.open) {
    clearInterval(liveTimer);
    dialog.close();
  }
}
setInterval(checkLive, 2000);
checkLive();

function refreshShot() {
  const next = new Image();
  next.onload = () => { $('live-shot').src = next.src; };
  next.src = `/api/live/shot?t=${Date.now()}`;
}

// One after the other, in the order sent: keys typed fast stay in order.
let liveQueue = Promise.resolve();
const liveSend = (path, body) => {
  liveQueue = liveQueue.then(() => api('POST', path, body)).then(refreshShot).catch((err) => { $('live-reason').textContent = err.message; });
  return liveQueue;
};

// A click on the picture is a click on the page, at the same spot of it.
$('live-shot').addEventListener('click', (e) => {
  const img = e.currentTarget;
  if (!img.naturalWidth) return;
  // Where in the picture, as a share of its box on the screen: the mouse and the box measured
  // alike, under the page's zoom of 90 % — offsetX and clientWidth are not.
  const box = img.getBoundingClientRect();
  const x = ((e.clientX - box.left) / box.width) * img.naturalWidth;
  const y = ((e.clientY - box.top) / box.height) * img.naturalHeight;
  liveSend('/api/live/click', { x, y });
  $('live-keys').focus({ preventScroll: true });
});
// The mouse wheel over the picture scrolls the page, at the same spot: what turned in the last
// moment at once — a touchpad's stream of small turns goes on as it comes, not at its end.
let wheel = 0;
let wheelTimer = null;
$('live-shot').addEventListener('wheel', (e) => {
  e.preventDefault();
  wheel += e.deltaY;
  const img = e.currentTarget;
  const box = img.getBoundingClientRect();
  const x = ((e.clientX - box.left) / box.width) * img.naturalWidth;
  const y = ((e.clientY - box.top) / box.height) * img.naturalHeight;
  if (wheelTimer) return;
  wheelTimer = setTimeout(() => {
    const dy = wheel;
    wheel = 0;
    wheelTimer = null;
    if (img.naturalWidth && dy) {
      liveSend('/api/live/scroll', { x, y, dy });
      setTimeout(refreshShot, 150);
    }
  }, 80);
}, { passive: false });
// The keyboard types into the page: the keys that do something there go on as keys, what they
// write as text — a phone's keyboard, which tells no keys, through the input's events.
const LIVE_KEYS = new Set(['Enter', 'Tab', 'Backspace', 'Delete', 'ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End', 'PageUp', 'PageDown']);
// Safari on an iPad names the arrows its own way; some keyboards tell Backspace by its code only.
const LIVE_ALIASES = { UIKeyInputLeftArrow: 'ArrowLeft', UIKeyInputRightArrow: 'ArrowRight', UIKeyInputUpArrow: 'ArrowUp', UIKeyInputDownArrow: 'ArrowDown' };
$('live-keys').addEventListener('keydown', (e) => {
  const key = LIVE_ALIASES[e.key] ?? (e.keyCode === 8 ? 'Backspace' : e.key);
  if (e.isComposing || e.ctrlKey || e.metaKey || e.altKey || !LIVE_KEYS.has(key)) return;
  e.preventDefault();
  liveSend('/api/live/key', { key });
});
// The input keeps one character out of sight, so that a phone's backspace has something to
// delete — on an empty input it tells nothing. Whatever else is in it went on to the page.
const LIVE_KEPT = '\u200b';
function liveReset() {
  const box = $('live-keys');
  box.value = LIVE_KEPT;
  box.setSelectionRange(1, 1);
}
function liveTyped() {
  const value = $('live-keys').value;
  liveReset();
  if (!value.startsWith(LIVE_KEPT)) liveSend('/api/live/key', { key: 'Backspace' });
  else if (value.length > LIVE_KEPT.length) liveSend('/api/live/type', { text: value.slice(LIVE_KEPT.length) });
}
$('live-keys').addEventListener('input', (e) => { if (!e.isComposing) liveTyped(); });
// A phone's keyboard writing a word at once: when it is done.
$('live-keys').addEventListener('compositionend', liveTyped);
$('live-keys').addEventListener('focus', liveReset);
// Pasted, as a whole: a password from a password manager.
$('live-keys').addEventListener('paste', (e) => {
  e.preventDefault();
  const text = e.clipboardData?.getData('text') ?? '';
  if (text) liveSend('/api/live/type', { text });
});
// The text field below the page: into the page's field — the one with the cursor, or the first — then emptied.
$('live-text-form').addEventListener('submit', (e) => {
  e.preventDefault();
  // Typed into the page's field, then sent there as Enter does: a code, a password, done.
  const text = $('live-text').value;
  if (text) {
    liveSend('/api/live/type', { text });
    liveSend('/api/live/key', { key: 'Enter' });
  }
  $('live-text').value = '';
});
$('live-clear').addEventListener('click', () => { liveSend('/api/live/clear'); $('live-keys').focus({ preventScroll: true }); });
for (const b of document.querySelectorAll('[data-live-key]')) {
  b.addEventListener('click', () => { liveSend('/api/live/key', { key: b.dataset.liveKey }); $('live-keys').focus({ preventScroll: true }); });
}
// The stored password into the field clicked last: the person's say when, never shown.
$('live-fill').addEventListener('click', () => { liveSend('/api/live/fill'); $('live-keys').focus({ preventScroll: true }); });
$('live-cancel').addEventListener('click', () => liveSend('/api/live/cancel'));
// Escape would close the dialog with the run still waiting: cancel it instead, asked.
$('live').addEventListener('cancel', async (e) => {
  e.preventDefault();
  if (await ask(T('Lauf abbrechen?', 'Cancel the run?'), { yes: T('Lauf abbrechen', 'Cancel the run'), no: T('Weiter warten', 'Keep waiting') })) liveSend('/api/live/cancel');
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
    // Held back by the login gate: run anyway only if asked and said yes.
    if (err.data?.gated && body && await ask(`${err.message}\n\n${T('Trotzdem anmelden?', 'Log in anyway?')}`, { yes: T('Trotzdem anmelden', 'Log in anyway') })) {
      return start(name, method, path, { ...body, force: true }, onDone);
    }
    tell(err.message);
  }
}

// Apple's and the further senders' receipts by month: to look at, or sent after asking. The month as on
// the command line, the current one at first.
const NAMES = T({ amazon: 'Amazon-Rechnungen', apple: 'Apple-Rechnungen', vodafone: 'Vodafone-Rechnungen', mail: 'Belege weiterer Absender' },
  { amazon: 'Amazon invoices', apple: 'Apple invoices', vodafone: 'Vodafone invoices', mail: "further senders' receipts" });
const monthOf = (kind) => document.querySelector(`.receipt-month[data-for="${kind}"]`).value;
{
  const now = new Date();
  for (const input of document.querySelectorAll('.receipt-month')) {
    input.value = input.placeholder = `${now.getMonth() + 1}/${now.getFullYear()}`;
  }
  // Amazon: the month it fetches — on a month's last day its own, the books closed, else the one before.
  const lastDay = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1).getDate() === 1;
  const last = new Date(now.getFullYear(), now.getMonth() - (lastDay ? 0 : 1), 1);
  for (const input of document.querySelectorAll('[data-last-month]')) input.value = input.placeholder = `${last.getMonth() + 1}/${last.getFullYear()}`;
}
for (const b of document.querySelectorAll('[data-view]')) {
  b.addEventListener('click', () => start(b.dataset.view, 'POST', `/api/${b.dataset.view}/view`, { month: monthOf(b.dataset.view) }));
}
for (const b of document.querySelectorAll('[data-send]')) {
  b.addEventListener('click', async () => {
    const kind = b.dataset.send;
    if (!(await ask(T(`${NAMES[kind]} ${monthOf(kind)} senden an ${mailTo()}?`, `Send ${NAMES[kind]} ${monthOf(kind)} to ${mailTo()}?`), { yes: T('Senden', 'Send') }))) return;
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
$('travel-send').addEventListener('click', async () => {
  const t = plan?.totals ?? {};
  if (!(await ask(T(`Reisekosten ${MONTHS[month.getMonth()]} ${month.getFullYear()} senden an ${mailTo()}?\n${t.days ?? 0} Reisetage, ${euro(totalOf(t))}`,
    `Send the travel expenses of ${MONTHS[month.getMonth()]} ${month.getFullYear()} to ${mailTo()}?\n${t.days ?? 0} travel days, ${euro(totalOf(t))}`), { yes: T('Senden', 'Send') }))) return;
  start('travel', 'POST', '/api/travel/send', { month: monthKey() });
});

// ── Customer invoice ───────────────────────────────────────────────────────

// Manuelle Eingabe: the period and the hours typed, or filled from a dropped PDF; the draft is for
// the client, article and reference of the settings, beneath on the same card.
dropZone($('upload-form'), sendReport);
$('report').addEventListener('change', () => { if ($('report').files[0]) sendReport($('report').files[0]); });

/** The current month, until something else is entered. */
function draftDefaults() {
  const now = new Date();
  if (!$('d-from').value) $('d-from').value = isoDay(new Date(now.getFullYear(), now.getMonth(), 1));
  if (!$('d-to').value) $('d-to').value = isoDay(new Date(now.getFullYear(), now.getMonth() + 1, 0));
}
draftDefaults();

/** The sevDesk client the draft is for: as saved in the settings. */
function draftClient() {
  const saved = settings?.saved ? JSON.parse(settings.saved) : {};
  return (saved.HarvestInvoice?.SevDesk?.ClientName ?? '').trim();
}

async function sendReport(file) {
  $('drop-text').textContent = T(`${file.name} wird gelesen…`, `Reading ${file.name}…`);
  const form = new FormData();
  form.append('report', file);
  let read;
  try {
    read = await api('POST', '/api/harvest/upload', form);
  } catch (err) {
    $('drop-text').textContent = err.message;
    return;
  }
  // What the PDF has, into the fields; a period it lacks stays as entered.
  if (read.from) $('d-from').value = read.from;
  if (read.to) $('d-to').value = read.to;
  $('d-hours').value = read.hours;
  $('drop-text').textContent = T(`${file.name}: ${hoursText(read.hours)} Stunden gelesen — bitte prüfen`, `${file.name}: ${hoursText(read.hours)} hours read — please check`);
}

$('draft-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const hours = Number($('d-hours').value);
  const from = $('d-from').value;
  const to = $('d-to').value;
  if (to < from) {
    tell(T('„Leistung bis“ liegt vor „Leistung von“.', '“Service to” is before “Service from”.'));
    return;
  }
  const client = draftClient();
  if (!client) {
    tell(T('Kein Kunde: „Kunde in sevDesk“ unter „Einstellungen Kundenrechnung“ eintragen und speichern.', 'No client: enter “Client in sevDesk” under “Invoice settings” and save.'));
    return;
  }
  if (!(await ask(T(`Rechnungsentwurf in sevDesk anlegen?\n${client}: ${dmy(from)} – ${dmy(to)}, ${hoursText(hours)} Stunden`,
    `Create the invoice draft in sevDesk?\n${client}: ${dmy(from)} – ${dmy(to)}, ${hoursText(hours)} hours`), { yes: T('Anlegen', 'Create') }))) return;
  start('invoice', 'POST', '/api/harvest/draft', { from, to, hours });
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
    tell(err.message);
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
const SCAN = T('Höchstens so viele Mails des Monats (je Ordner)', 'At most this many of the month\'s mails (per folder)');
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
  // Receipts mailed, copied to a storage too, by rclone set up on the Pi: iCloud Drive, Google Drive, …
  archive: [[T('Ablage (rclone)', 'Storage (rclone)'), [
    ['Archive.Enabled', T('Belege ablegen', 'Store receipts'), 'bool', false],
    ['Archive.Remote', T('rclone-Remote', 'rclone remote')],
    ['Archive.Folder', T('Ordner', 'Folder'), 'text', 'Belege/{YYYY}/{MM}'],
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
    ['HarvestInvoice.Harvest.Client', T('Kunde in Harvest', 'Client in Harvest'), 'shadow', 'HarvestInvoice.SevDesk.ClientName'],
    ['HarvestInvoice.Harvest.Token', T('Harvest-API-Token', 'Harvest API token'), 'secret', 3],
    BREAK,
    ['HarvestInvoice.SkipExisting', T('Entwurf nicht doppelt anlegen', 'No duplicate drafts'), 'bool', true],
  ]]],
  amazon: [['', [
    ['AmazonInvoice.Username', T('E-Mail-Adresse (Amazon Business)', 'Email address (Amazon Business)')], ['AmazonInvoice.Password', T('Passwort', 'Password'), 'secret', 5],
  ]]],
  apple: [['', [
    ['AppleInvoice.Filter.Count', SCAN, 'number', 1000],
    ['AppleInvoice.Filter.From', T('Absender-Domain', 'Sender domain'), 'text', 'apple.com'],
    ['AppleInvoice.Filter.Subject', T('Betreff', 'Subject'), 'text', 'Deine Rechnung von Apple'],
    ['AppleInvoice.Filter.Folders', FOLDERS, 'list', 'INBOX, Rechnungen'],
  ]]],
  advisor: [[T('Steuerberater', 'Tax adviser'), [
    ['Advisor.To', T('E-Mail-Adresse', 'Email address')],
    ['Advisor.Bcc', T('Blindkopie (BCC) an', 'Blind copy (BCC) to'), 'text', T('keine', 'none')],
    ['Advisor.Subject', T('Betreff', 'Subject'), 'text', 'Belege {MM}/{YYYY}'],
    ['Advisor.Text', T('Text', 'Text'), 'textarea', T('Hallo, alle Rechnungen & Belege sind für {MM}/{YYYY} in sevDesk gebucht. Viele Grüße', 'Hallo, alle Rechnungen & Belege sind für {MM}/{YYYY} in sevDesk gebucht. Viele Grüße')],
  ]]],
  mailrules: [['', [
    ['MailReceipts.Count', SCAN, 'number', 1000],
    ['MailReceipts.Folders', FOLDERS, 'list', 'INBOX, Rechnungen'],
  ]]],
  notify: [[T('Benachrichtigung (Pushover)', 'Notifications (Pushover)'), [
    ['Notify.Pushover.Enabled', T('Per Pushover benachrichtigen', 'Notify by Pushover'), 'bool', false],
    ['Notify.Pushover.Token', T('API-Token der Anwendung', "The application's API token"), 'secret', 6],
    ['Notify.Pushover.User', T('User-Key', 'User key'), 'secret', 7],
    ['Notify.Pushover.Device', T('Gerät', 'Device'), 'text', T('alle', 'all')],
    BREAK,
    ['Notify.Closing', T('Monatsabschluss: melden, was im Vormonat fehlt', 'Closing: tell what last month is missing'), 'bool', true],
    ['Notify.ClosingDay', T('Am Tag des Monats (9:00)', 'On the day of the month (9:00)'), 'number', 5],
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
    // A text of several lines — the mail to the tax adviser — in a box of its own.
    input = el(type === 'textarea' ? 'textarea' : 'input');
    if (type === 'textarea') input.rows = 6;
    else input.type = type === 'secret' ? 'password' : type === 'number' || type === 'money' ? 'number' : 'text';
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
    if ((type === 'text' || type === 'number' || type === 'list' || type === 'textarea') && extra !== undefined) input.placeholder = `${DEFAULT}: ${extra}`;
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
    // As the page edits them: no mailboxes are an empty list — else the first edit anywhere
    // would count the settings as changed.
    if (settings.config.Mail) settings.config.Mail.Mailboxes ??= [];
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
    if (part === 'archive') {
      children.push(el('p', 'muted', T('Gesendete Belege (Amazon, Apple, Vodafone, weitere Absender) werden zusätzlich per rclone abgelegt – iCloud Drive, Google Drive, OneDrive, Nextcloud oder jeder andere Speicher, für den auf dem Pi ein rclone-Remote eingerichtet ist (rclone config). Im Ordner stehen {YYYY} und {MM} für Jahr und Monat des Belegs. Vorhandene Dateien bleiben, die neue heißt dann „… (1).pdf“.',
        'Receipts sent (Amazon, Apple, Vodafone, further senders) are also stored with rclone – iCloud Drive, Google Drive, OneDrive, Nextcloud or any other storage with an rclone remote set up on the Pi (rclone config). In the folder, {YYYY} and {MM} stand for the receipt\'s year and month. Files there are kept; the new one is then called “… (1).pdf”.')));
    }
    if (part === 'web') {
      children.push(el('p', 'muted', T('Nur im Heimnetz: nur Geräte mit privater Adresse (192.168.…, 10.…, localhost), auch bei versehentlich offenem Port. Aus: auch von außen, etwa über Caddy — aber nur, wenn ein Mitarbeiter ein Passwort hat, und nur mit Anmeldung. Hinter Caddy zählt die Adresse des Besuchers; Caddy in Docker (172.…) braucht das Proxy-Token, dasselbe wie im Caddyfile.',
        'Home network only: only devices with a private address (192.168.…, 10.…, localhost), even with the port open by mistake. Off: from outside too, through Caddy say — but only once an employee has a password, and only after logging in. Behind Caddy the visitor\'s address counts; Caddy in Docker (172.…) needs the proxy token, the same as in the Caddyfile.')));
    }
    if (part === 'mail') children.push(renderMailboxes(cfg));
    if (part === 'mailrules') children.unshift(renderRules(cfg));
    if (part === 'advisor') {
      children.push(el('p', 'muted', T('„Steuerberater informieren“ auf Abschluss schickt diese Mail – nur an ihn, nicht an die Autobox; als Blindkopie an die Adresse darunter, wenn eine eingetragen ist. {MM}, {YYYY} und {MONAT} stehen für den Monat. Auf Wunsch hängt ein ZIP mit allen Belegen des Monats aus der Ablage (rclone) an.',
        '“Inform tax adviser” on Closing sends this mail – to him only, not to the Autobox; a blind copy to the address beneath, if one is set. {MM}, {YYYY} and {MONAT} stand for the month. If asked, a zip of all of the month\'s receipts from the storage (rclone) comes with it.')));
    }
    if (part === 'notify') {
      children.push(el('p', 'muted', T('Aufs Handy kommt: ein Lauf aus dem Zeitplan, der fehlschlägt, und am Abschlusstag, welche Belege im Vormonat noch fehlen – nur von Quellen mit Zeitplan, die weder etwas gesendet noch nachgesehen haben. Die Anwendung legst du unter pushover.net/apps an; der User-Key steht auf pushover.net nach der Anmeldung.',
        'To the phone: a scheduled run that fails, and on the closing day which receipts last month is still missing – only of sources with a schedule that neither sent nor looked. Create the application at pushover.net/apps; the user key is on pushover.net once logged in.')));
      children.push(el('div', 'actions', button(T('Test senden', 'Send test'), '', async (b) => {
        b.disabled = true;
        try {
          await api('POST', '/api/notify/test');
          tell(T('Gesendet – kommt sie auf dem Handy an?', 'Sent – does it arrive on the phone?'));
        } catch (err) {
          tell(err.message);
        } finally {
          b.disabled = false;
        }
      })));
    }
    if (part === 'mail') {
      children.push(el('p', 'muted', T(`Gespeichert in ${settings.path}, die vorige Fassung daneben als config.yaml.bak. Weitere Einstellungen beim jeweiligen Bereich: Kunden bei den Reisekosten, Harvest und sevDesk bei der Kundenrechnung, Amazon, Apple, Vodafone und weitere Absender bei den Belegen, Pushover hier unten.`,
        `Saved in ${settings.path}, the version before beside it as config.yaml.bak. Further settings are with their section: clients with Travel, Harvest and sevDesk with Invoice, Amazon, Apple, Vodafone and further senders with Receipts, Pushover below.`)));
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
      tell(T('Das Logo muss ein PNG oder JPEG sein.', 'The logo must be a PNG or JPEG.'));
      return;
    }
    if (file.size > 2 << 20) {
      tell(T('Das Logo ist größer als 2 MB.', 'The logo is larger than 2 MB.'));
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
  dropZone(card, take);

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
 * The further senders' rules: a name, the sender's domains, and optionally what the subject and
 * the PDF's name contain. Saved with the button at the bottom.
 */
function renderRules(cfg) {
  cfg.MailReceipts ??= {};
  cfg.MailReceipts.Rules ??= [];
  const rules = cfg.MailReceipts.Rules;
  const changed = () => { renderSettings(); refreshSave(); };
  const input = (rule, key, cls, placeholder, label) => {
    const i = el('input', cls);
    i.value = rule[key] ?? '';
    i.placeholder = placeholder;
    i.autocomplete = 'off';
    i.spellcheck = false;
    i.setAttribute('aria-label', label);
    i.addEventListener('input', () => { rule[key] = i.value.trim(); refreshSave(); });
    return i;
  };
  const list = el('ul', 'rows staff rules', ...rules.map((rule, i) => {
    const on = el('input');
    on.type = 'checkbox';
    on.checked = !rule.Off;
    on.title = T('an', 'on');
    on.setAttribute('aria-label', T('Regel an', 'Rule on'));
    on.addEventListener('change', () => { rule.Off = !on.checked; refreshSave(); });
    return el('li', '', on,
      input(rule, 'Name', 'grow', T('Name, z. B. Strato', 'Name, e.g. Strato'), 'Name'),
      input(rule, 'From', 'grow', T('Absender-Domain, z. B. strato.de', 'Sender domain, e.g. strato.de'), T('Absender-Domains', 'Sender domains')),
      input(rule, 'Subject', 'grow', T('Betreff enthält', 'Subject contains'), T('Betreff enthält', 'Subject contains')),
      input(rule, 'Attachment', 'grow', T('PDF-Name enthält', 'PDF name contains'), T('PDF-Name enthält', 'PDF name contains')),
      button(T('entfernen', 'remove'), 'link', () => { rules.splice(i, 1); changed(); }));
  }));
  return el('div', '',
    el('p', 'muted', T('Je Regel: Mails von diesen Absender-Domains (mehrere mit Komma), deren Betreff das Wort enthält, mit ihren PDF-Anhängen – benannt wie die übrigen Belege, 11_2026_Rechnung_Strato_03112026.pdf. Was schon gesendet ist, geht nicht noch einmal. Betreff und PDF-Name leer: jede Mail, jedes PDF. Für den Zeitplan: „Weitere Absender abholen“.',
      'Per rule: mails from these sender domains (several with commas) whose subject contains the word, with their PDF attachments – named as the other receipts, 11_2026_Rechnung_Strato_03112026.pdf. What was sent already does not go again. Subject and PDF name empty: any mail, any PDF. For the schedule: “Fetch further senders”.')),
    rules.length ? list : el('p', 'muted', T('Noch keine Regel.', 'No rule yet.')),
    el('div', 'actions', button(T('Regel hinzufügen', 'Add rule'), '', () => { rules.push({ Name: '', From: '' }); changed(); })));
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

// ── Closing: a month's books, source by source ─────────────────────────────

// The month shown: until the closing day the one before, being closed; after it the current one.
let closingMonth = null;

async function loadClosing() {
  let data;
  const key = closingMonth && `${closingMonth.getFullYear()}-${String(closingMonth.getMonth() + 1).padStart(2, '0')}`;
  try {
    data = await api('GET', `/api/closing${key ? `?month=${key}` : ''}`);
  } catch (err) {
    $('closing').replaceChildren(el('li', 'badge failed', err.message));
    return;
  }
  if (!closingMonth) {
    const now = new Date();
    closingMonth = new Date(now.getFullYear(), now.getMonth() - (now.getDate() <= data.closingDay ? 1 : 0), 1);
    return loadClosing();
  }
  renderClosing(data);
}
loaders.closing = loadClosing;

$('closing-prev').addEventListener('click', () => { closingMonth.setMonth(closingMonth.getMonth() - 1); loadClosing(); });
$('closing-next').addEventListener('click', () => { closingMonth.setMonth(closingMonth.getMonth() + 1); loadClosing(); });

// The receipts taken out of the ledger while the page is open: shown struck through, as going
// again with the next run — gone from the ledger, they would else be gone from the list.
const unsent = [];

/** (×): the receipt out of the ledger, at once — several in a row, the next run sends them all. */
function unsendButton(row, doc, id) {
  const b = button('×', 'link unsend', async () => {
    b.disabled = true;
    try {
      await api('DELETE', '/api/sent', { source: row.source, id });
      unsent.push({ source: row.source, name: doc.name, month: doc.month });
    } catch (err) {
      tell(err.message);
    }
    loadClosing();
  });
  b.title = T('Erneut senden: aus dem Verzeichnis nehmen – der nächste Lauf schickt den Beleg noch einmal', 'Send again: take it out of the ledger – the next run sends it once more');
  b.setAttribute('aria-label', b.title);
  return b;
}

const shortWhen = (iso) => new Date(iso).toLocaleString(LOCALE, { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });

let closingData = null;

const STATE = () => T({ done: 'Abgeschlossen: alle Belege da, Rechnung in sevDesk', progress: 'In Arbeit', none: 'Noch nichts passiert' },
  { done: 'Closed: all receipts in, invoice in sevDesk', progress: 'Under way', none: 'Nothing has happened yet' });

function renderClosing(data) {
  closingData = data;
  const name = `${MONTHS[closingMonth.getMonth()]} ${closingMonth.getFullYear()}`;
  $('closing-title').textContent = name;
  const state = $('closing-state');
  state.dataset.state = data.state;
  state.title = STATE()[data.state];
  state.setAttribute('aria-label', STATE()[data.state]);
  renderAdvisor(data);
  const open = data.rows.filter((r) => r.open).map((r) => r.source);
  const expected = data.rows.filter((r) => r.expected).length;
  const day = `${data.closingDay}.`;
  $('closing-note').textContent = [
    open.length ? T(`Noch offen: ${open.join(', ')}.`, `Still open: ${open.join(', ')}.`)
      : expected ? T(`Alle ${expected} Quellen mit Zeitplan sind erledigt.`, `All ${expected} sources with a schedule are done.`) : '',
    data.notify ? T(`Am ${day} des Folgemonats um 9:00 kommt per Pushover, was dann noch fehlt.`, `On the ${data.closingDay}th of the next month at 9:00 Pushover tells what is still missing.`)
      : T(`Mit Pushover (Einstellungen) käme am ${day} des Folgemonats eine Nachricht, wenn noch etwas fehlt.`, `With Pushover (Settings) a message would come on the ${data.closingDay}th of the next month if something is missing.`),
  ].filter(Boolean).join(' ');

  $('closing').replaceChildren(...data.rows.map((r) => {
    const sent = r.docs.length;
    // Filed by hand: found in the storage, booked by hand rather than sent.
    const filed = r.docs.filter((d) => d.filed).length;
    const counted = [sent - filed && T(`${sent - filed} gesendet`, `${sent - filed} sent`), filed && T(`${filed} von Hand`, `${filed} by hand`)].filter(Boolean).join(', ');
    const state = sent ? el('span', 'badge ok', r.source === 'Kundenrechnung' ? T('Entwurf angelegt', 'draft created') : counted)
      : !never(r.checked) ? el('span', 'badge ok', T('nichts gefunden', 'nothing found'))
        : r.open ? el('span', 'badge failed', T('offen', 'open'))
          : el('span', 'badge', r.expected ? '—' : T('ohne Zeitplan', 'no schedule'));
    const lines = [];
    for (const d of r.docs) {
      const id = d.hash || d.name;
      // Filed by hand: said on a line of its own beneath the name, the date being the finding's.
      const filed = d.filed && el('span', 'filed muted', d.filed === d.name ? T('von Hand abgelegt', 'filed by hand') : T(`von Hand abgelegt als ${d.filed}`, `filed by hand as ${d.filed}`));
      lines.push(el('div', 'doc', el('span', 'grow', d.name), !d.filed && el('span', 'muted', shortWhen(d.at)),
        // A draft in sevDesk is made once; a receipt taken out goes again with the next run.
        r.job !== 'harvest-invoice' && unsendButton(r, d, id), filed));
    }
    for (const d of unsent.filter((u) => u.source === r.source && u.month === data.month)) {
      lines.push(el('div', 'doc unsent', el('span', 'grow', d.name), el('span', 'muted', T('geht beim nächsten Lauf erneut', 'goes again with the next run'))));
    }
    if (!sent && !never(r.checked)) lines.push(el('div', 'muted', T(`Nachgesehen ${shortWhen(r.checked)}, kein Beleg für den Monat.`, `Looked ${shortWhen(r.checked)}, no receipt for the month.`)));
    if (r.failed) lines.push(el('div', 'badge failed', `${T('Letzter Lauf fehlgeschlagen', 'Last run failed')} ${never(r.failedAt) ? '' : shortWhen(r.failedAt)}: ${r.failed}`));
    if (r.expected && !never(r.next)) lines.push(el('div', 'muted', `${T('Nächster Lauf', 'Next run')}: ${shortWhen(r.next)}`));
    const link = el('a', 'source', r.source);
    link.href = r.job === 'travel-expense' ? '#travel' : r.job === 'harvest-invoice' ? '#invoice' : '#receipts';
    return el('li', r.open ? 'open' : '', el('div', 'closing-head', link, state), lines.length > 0 && el('div', 'closing-lines small', ...lines));
  }));
}

/** The tax adviser's mail of the month as it will go, and whether and when it went. */
function renderAdvisor(data) {
  const a = data.advisor;
  $('advisor-note').textContent = !a.to ? T('Noch keine Adresse: unter Einstellungen › Steuerberater eintragen.', 'No address yet: enter it under Settings › Tax adviser.')
    : !never(a.informed) ? T(`Informiert am ${shortWhen(a.informed)}.`, `Informed on ${shortWhen(a.informed)}.`)
      : T('Noch nicht informiert.', 'Not informed yet.');
  $('advisor-send').disabled = !a.to;
  $('advisor-zip-label').hidden = !a.zip;
  if (!a.zip) $('advisor-zip').checked = false;
}

$('advisor-send').addEventListener('click', async () => {
  const data = closingData;
  if (!data) return;
  const a = data.advisor;
  const zip = $('advisor-zip').checked;
  const open = data.rows.filter((r) => r.open).map((r) => r.source);
  const lines = [T(`„${a.subject}“ an ${a.to} senden${zip ? ', mit allen Belegen des Monats als ZIP' : ''}?`, `Send “${a.subject}” to ${a.to}${zip ? ', with all of the month\'s receipts as zip' : ''}?`)];
  if (open.length) lines.push(T(`Achtung, noch offen: ${open.join(', ')}.`, `Mind, still open: ${open.join(', ')}.`));
  if (!never(a.informed)) lines.push(T(`Schon informiert am ${shortWhen(a.informed)}.`, `Informed already on ${shortWhen(a.informed)}.`));
  if (!(await ask(lines.join('\n\n'), { yes: T('Senden', 'Send') }))) return;
  start('advisor', 'POST', '/api/advisor/send', { month: data.month, zip }, () => loadClosing());
});

// ── Start ──────────────────────────────────────────────────────────────────

loadOverview();
showTab();
setInterval(loadOverview, 10_000);
