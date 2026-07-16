document.addEventListener('DOMContentLoaded', () => {
  formatTimestamps();
  showToast();
});

function formatTimestamps() {
  const els = document.querySelectorAll('time[data-timestamp]');
  if (!els.length) return;

  const fmt = new Intl.DateTimeFormat(navigator.language, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: 'numeric',
    minute: 'numeric',
  });

  els.forEach(el => {
    const ts = Number(el.dataset.timestamp);
    if (Number.isNaN(ts)) return;
    el.textContent = fmt.format(new Date(ts));
  });
}

function showToast() {
  const toastEl = document.getElementById('toast');
  if (toastEl && typeof bootstrap !== 'undefined') {
    new bootstrap.Toast(toastEl).show();
  }
}
