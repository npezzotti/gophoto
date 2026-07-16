const POLLING_INTERVAL = 2000; // ms between status checks
const MAX_POLLS = 60;          // ~2 minutes before timeout

const photoUploadForm    = document.getElementById('photo-upload-form');
const addPhotoModalEl    = document.getElementById('addPhotoModal');
const progressModalEl    = document.getElementById('photoProgressModal');
const progressBar        = progressModalEl.querySelector('[role="progressbar"]');

const addPhotoModal  = bootstrap.Modal.getOrCreateInstance(addPhotoModalEl);
const progressModal  = bootstrap.Modal.getOrCreateInstance(progressModalEl);

photoUploadForm.addEventListener('submit', async e => {
  e.preventDefault();
  const form = e.target;
  const submitButton = form.querySelector('[type="submit"]');
  if (submitButton) submitButton.disabled = true;

  try {
    const response = await uploadPhoto(form);
    if (!response.ok) {
      const body = await response.json().catch(() => ({}));
      throw new Error(body.error || `Upload failed: HTTP ${response.status}`);
    }

    const data = await response.json();
    if (!data.id) throw new Error('No photo ID returned from server');

    // Hide the upload modal; once it's gone, show the progress modal.
    addPhotoModalEl.addEventListener('hidden.bs.modal', () => {
      resetProgress();
      progressModal.show();
    }, { once: true });

    progressModalEl.addEventListener('shown.bs.modal', () => {
      startPolling(data.id);
    }, { once: true });

    addPhotoModal.hide();

  } catch (err) {
    console.error('Error uploading photo:', err);
    showFormError(form, err.message);
    if (submitButton) submitButton.disabled = false;
  }
});

function startPolling(photoId) {
  let pollCount = 0;

  async function tick() {
    if (++pollCount > MAX_POLLS) {
      progressModal.hide();
      console.error('Photo processing timed out');
      return;
    }

    try {
      const done = await checkPhotoStatus(photoId);
      if (done) {
        // Give the user a moment to see 100% before the reload.
        setTimeout(() => location.reload(), 1000);
      } else {
        setTimeout(tick, POLLING_INTERVAL);
      }
    } catch (err) {
      progressModal.hide();
      console.error('Error polling photo status:', err);
    }
  }

  setTimeout(tick, POLLING_INTERVAL);
}

async function uploadPhoto(form) {
  return fetch(form.action, { method: form.method, body: new FormData(form) });
}

async function fetchPhotoStatus(photoId) {
  return fetch(`/photos/status?id=${photoId}`);
}

async function checkPhotoStatus(photoId) {
  const response = await fetchPhotoStatus(photoId);
  if (!response.ok) {
    throw new Error(`Status check failed: HTTP ${response.status}`);
  }
  const data = await response.json();

  switch (data.status) {
    case 'processing':
      advanceProgress();
      return false;
    case 'processed':
      setProgress(100);
      return true;
    case 'errored':
      throw new Error('Photo processing failed on the server');
    default:
      throw new Error(`Unknown photo status: ${data.status}`);
  }
}

function resetProgress() {
  setProgress(0);
}

function advanceProgress() {
  const max     = parseInt(progressBar.getAttribute('aria-valuemax'), 10) || 100;
  const current = parseInt(progressBar.getAttribute('aria-valuenow'), 10) || 0;
  // Advance by 10 but cap at max - 1 so we never falsely show 100%.
  setProgress(Math.min(current + 10, max - 1));
}

function setProgress(percent) {
  progressBar.setAttribute('aria-valuenow', percent);
  progressBar.style.width = `${percent}%`;
}

function showFormError(form, message) {
  let alert = form.querySelector('.upload-error');
  if (!alert) {
    alert = document.createElement('p');
    alert.className = 'upload-error text-danger mt-2';
    form.appendChild(alert);
  }
  alert.textContent = message;
}
