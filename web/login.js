'use strict';

// The login: an employee and their password — and, from outside with an authenticator set up,
// its code, once the server asks for it. Without anyone having a password, or logged in already,
// straight on to the page.
const $ = (id) => document.getElementById(id);
const LANG = document.documentElement.dataset.lang === 'en' ? 'en' : 'de';

if (LANG === 'en') document.title = 'Login · feed-my-accounting';

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

// codeStep shows the screen for the code, or back the one for name and password. The name and
// password stay as typed: sent again with the code.
function codeStep(on) {
  $('step-password').hidden = on;
  $('step-code').hidden = !on;
  $('code').required = on;
  $('code').value = '';
  $('error').hidden = true;
  (on ? $('code') : $('password')).focus();
}
$('back').addEventListener('click', (e) => { e.preventDefault(); codeStep(false); });

$('login').addEventListener('submit', async (e) => {
  e.preventDefault();
  $('error').hidden = true;
  const res = await fetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Lang': LANG },
    body: JSON.stringify({ name: $('name').value, password: $('password').value, code: $('code').value }),
  });
  if (res.ok) {
    location.replace('/');
    return;
  }
  const data = await res.json().catch(() => ({}));
  // The password was right; the authenticator's code is asked for, on the next screen.
  if (data.code && $('step-code').hidden) {
    codeStep(true);
    return;
  }
  $('error').textContent = data.error || (LANG === 'en' ? `Login failed: HTTP ${res.status}` : `Anmelden ging nicht: HTTP ${res.status}`);
  $('error').hidden = false;
  (data.code ? $('code') : $('password')).select();
});
