const toastElList = document.querySelectorAll('.toast')
const toastList = [...toastElList].map(toastEl => new bootstrap.Toast(toastEl).show())


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
