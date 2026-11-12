// The language before the first paint, as rummeyer.com chooses it: ?lang= in the address, else
// the one chosen before, else the browser's — German or English, German if neither. A file of its
// own, as the page allows no inline script.
(() => {
  const supported = ['de', 'en'];
  const requested = new URLSearchParams(location.search).get('lang');
  let stored = null;
  try { stored = localStorage.getItem('lang'); } catch { /* private window */ }
  const browser = (navigator.language || 'de').slice(0, 2).toLowerCase();
  let lang = 'de';
  for (const candidate of [browser, stored, requested]) {
    if (supported.includes(candidate)) lang = candidate;
  }
  if (supported.includes(requested)) {
    try { localStorage.setItem('lang', requested); } catch { /* private window */ }
  }
  document.documentElement.lang = lang;
  document.documentElement.dataset.lang = lang;
})();
