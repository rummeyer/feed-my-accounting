'use strict';

// The login: an employee and their password. Without anyone having a password, or logged in
// already, straight on to the page.
const $ = (id) => document.getElementById(id);
const LANG = document.documentElement.dataset.lang === 'en' ? 'en' : 'de';

// The language switch, as on the page: loads the login anew in the other one.
for (const link of document.querySelectorAll('[data-lang-link]')) {
  link.setAttribute('aria-current', String(link.dataset.langLink === LANG));
  link.addEventListener('click', (e) => {
    e.preventDefault();
    if (link.dataset.langLink !== LANG) location.href = `?lang=${link.dataset.langLink}`;
  });
}
if (LANG === 'en') document.title = 'Log in · feed-my-accounting';

(async () => {
  const session = await fetch('/api/session', { headers: { 'X-Lang': LANG } }).then((r) => r.json()).catch(() => null);
  if (session && (!session.required || session.user)) {
    location.replace('/');
    return;
  }
  const names = session?.names ?? [];
  // From outside no list of employees: the name is typed.
  if (names.length === 0) {
    $('name').remove();
    $('name-typed').id = 'name';
    $('name').hidden = false;
    $('name').required = true;
  }
  for (const name of names) {
    const o = document.createElement('option');
    o.value = o.textContent = name;
    $('name').append(o);
  }
  // At home the name is enough: no password asked.
  const password = session?.password !== false;
  $('password-label').hidden = !password;
  $('password').required = password;
  (password ? $('password') : $('name')).focus();
})();

$('login').addEventListener('submit', async (e) => {
  e.preventDefault();
  $('error').hidden = true;
  const res = await fetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Lang': LANG },
    body: JSON.stringify({ name: $('name').value, password: $('password').value }),
  });
  if (res.ok) {
    location.replace('/');
    return;
  }
  const data = await res.json().catch(() => ({}));
  $('error').textContent = data.error || (LANG === 'en' ? `Logging in failed: HTTP ${res.status}` : `Anmelden ging nicht: HTTP ${res.status}`);
  $('error').hidden = false;
  $('password').select();
});
