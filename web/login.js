'use strict';

// The login: an employee and their password. Without anyone having a password, or logged in
// already, straight on to the page.
const $ = (id) => document.getElementById(id);

(async () => {
  const session = await fetch('/api/session').then((r) => r.json()).catch(() => null);
  if (session && (!session.required || session.user)) {
    location.replace('/');
    return;
  }
  for (const name of session?.names ?? []) {
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
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: $('name').value, password: $('password').value }),
  });
  if (res.ok) {
    location.replace('/');
    return;
  }
  const data = await res.json().catch(() => ({}));
  $('error').textContent = data.error || `Anmelden ging nicht: HTTP ${res.status}`;
  $('error').hidden = false;
  $('password').select();
});
