const photoUploadForm = document.getElementById('photo-upload-form');
photoUploadForm.addEventListener('submit', async e => {
  e.preventDefault();
  const form = e.target;
  const submitButton = form.querySelector('[type="submit"]');
  if (submitButton) submitButton.disabled = true;

  try {
    const formData = new FormData(form);
    const response = await fetch(form.action, {
      method: form.method,
      body: formData
    });

    if (response.ok) {
      const data = await response.json();
      processingPhotoId = data.id;

      if (!processingPhotoId) {
        throw new Error("No photo ID returned from server");
      }

      const addPhotoModelEl = document.getElementById('addPhotoModal')
      const addPhotoModal = bootstrap.Modal.getOrCreateInstance(addPhotoModelEl, {});
      const photoProgressModalEl = document.getElementById('photoProgressModal')
      const progressModal = bootstrap.Modal.getOrCreateInstance(photoProgressModalEl, {});

      addPhotoModelEl.addEventListener('hidden.bs.modal', () => {
        progressModal.show();
      }, { once: true });

      addPhotoModal.hide();

      setTimeout(() => {
        const progressBar = photoProgressModalEl.querySelector('[role="progressbar"]');
        const pollInterval = setInterval(async () => {
          if (progressBar) {
            let currentValue = parseInt(progressBar.getAttribute('aria-valuenow')) || 0;
            let maxValue = parseInt(progressBar.getAttribute('aria-valuemax')) || 100;
            currentValue = Math.min(currentValue + 10, maxValue);
            progressBar.setAttribute('aria-valuenow', currentValue);
            progressBar.style.width = `${currentValue}%`;
          }

          try {
            const response = await fetch(`/photo/status?id=${processingPhotoId}`)
            const data = await response.json()
            switch (data.status) {
              case "processing":
                // Still processing, do nothing
                break;
              case "processed":
                // Complete the progress bar
                if (progressBar) {
                  progressBar.setAttribute('aria-valuenow', 100);
                  progressBar.style.width = `100%`;
                }
                clearInterval(pollInterval);

                // Refresh the page after a short delay to show the new photo
                setTimeout(() => {
                  location.reload();
                }, 1000);
                break;
              case "errored":
                throw new Error("Error processing photo");
              default:
                throw new Error("Unknown photo status");
            }
          } catch (err) {
            console.error("Error fetching photo status:", err);
            clearInterval(pollInterval);
            progressModal.modal('hide');
          }
        }, 1000)
      }, 500);
    }
  } catch (err) {
    console.error("Error uploading photo:", err);
  }
});
