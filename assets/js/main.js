// Convert all <time> elements with a data-timestamp attribute to localized date strings
document.querySelectorAll('time[data-timestamp]').forEach(el => {
  const date = new Date(el.dataset.timestamp);
  el.textContent = date.toLocaleDateString([], {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: 'numeric',
    minute: 'numeric',
  });
});

const toastElList = document.querySelectorAll('.toast');
const toastList = [...toastElList].map(toastEl => new bootstrap.Toast(toastEl).show());

function showToast() {
  const toast = document.getElementById('toast');
  if (!toast) {
    return;
  }

  const toastInstance = new bootstrap.Toast(toast);
  toastInstance.show();
}

// Show the toast notification if it exists
showToast();
