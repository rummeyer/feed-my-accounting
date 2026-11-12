// The theme before the first paint, as rummeyer.com does it: light, dark or the system's. A file
// of its own, as the page allows no inline script.
try {
  const pref = localStorage.getItem('theme') || 'auto';
  document.documentElement.dataset.themePref = pref;
  if (pref !== 'auto') document.documentElement.dataset.theme = pref;
} catch {
  document.documentElement.dataset.themePref = 'auto';
}
