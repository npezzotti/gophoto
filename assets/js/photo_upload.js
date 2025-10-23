const POLLING_INTERVAL = 2000; // 2 seconds

const photoUploadForm = document.getElementById('photo-upload-form');
photoUploadForm.addEventListener('submit', async e => {
  e.preventDefault();
  const form = e.target;
  const submitButton = form.querySelector('[type="submit"]');
  if (submitButton) submitButton.disabled = true;

  try {
    const response = await uploadPhoto(form);
    if (response.ok) {
      const data = await response.json();

      processingPhotoId = data.id;
      if (!processingPhotoId) {
        throw new Error("No photo ID returned from server");
      }

      const addPhotoModelEl = document.getElementById('addPhotoModal');
      const addPhotoModal = bootstrap.Modal.getOrCreateInstance(addPhotoModelEl, {});
      const photoProgressModalEl = document.getElementById('photoProgressModal');
      const progressModal = bootstrap.Modal.getOrCreateInstance(photoProgressModalEl, {});

      addPhotoModelEl.addEventListener('hidden.bs.modal', () => {
        progressModal.show();
      }, { once: true });

      addPhotoModal.hide();

      setTimeout(() => {
        const progressBar = photoProgressModalEl.querySelector('[role="progressbar"]');
        const pollInterval = setInterval(async () => {
          try {
            await pollPhotoProcessingStatus(processingPhotoId, progressBar, pollInterval);
          } catch (err) {
            clearInterval(pollInterval);
            progressModal.hide();
            console.log("Error polling photo status:", err);
          }
        }, POLLING_INTERVAL);
      }, 500);
    } else {
      const resp = await response.json();
      errorMessage = resp.error || "Unknown error uploading photo";
      throw new Error(errorMessage);
    }
  } catch (err) {
    console.log("Error uploading photo: " + err);
  }
});

async function uploadPhoto(form) {
  const formData = new FormData(form);
  try {
    const response = await fetch(form.action, {
      method: form.method,
      body: formData,
    });
    return response;
  } catch (error) {
    throw error;
  }
}

async function fetchPhotoStatus(photoId) {
  try {
    const response = await fetch(`/api/photos/status?id=${photoId}`);
    return response;
  } catch (error) {
    throw error;
  }
}

function setProgress(bar, percent) {
  bar.setAttribute('aria-valuenow', percent);
  bar.style.width = `${percent}%`;
}

function getProgress(bar) {
  return parseInt(bar.getAttribute('aria-valuenow')) || 0;
}

async function pollPhotoProcessingStatus(photoId, progressBar, interval) {
  if (progressBar) {
    let currentValue = getProgress(progressBar);
    let maxValue = parseInt(progressBar.getAttribute('aria-valuemax')) || 100;
    currentValue = Math.min(currentValue + 10, maxValue);
    if (currentValue === maxValue) {
      currentValue = maxValue - 5; // Don't reach 100% until confirmed
    }

    setProgress(progressBar, currentValue);
  }

  try {
    const response = await fetchPhotoStatus(photoId);
    const data = await response.json();
    switch (data.status) {
      case "processing":
        // Still processing, do nothing
        break;
      case "processed":
        // Complete the progress bar
        if (progressBar) {
          setProgress(progressBar, 100);
        }

        // Stop polling
        clearInterval(interval);

        // Refresh the page after a short delay to show the new photo
        setTimeout(() => {
          location.reload();
        }, 1000);
        break;
      case "errored":
        throw new Error("Photo processing failed");
      default:
        throw new Error("Unknown photo status");
    }
  } catch (err) {
    throw new Error("Error fetching photo status: " + err);
  }
}
