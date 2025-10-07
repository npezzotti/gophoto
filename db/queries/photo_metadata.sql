-- name: GetPhotoMetadataByPhotoID :many
SELECT * FROM photo_metadata
WHERE photo_id = $1;

-- name: GetPhotoMetadataByPhotoIDAndVariant :one
SELECT * FROM photo_metadata
WHERE photo_id = $1 AND variant = $2
LIMIT 1;

-- name: CreatePhotoMetadata :one
INSERT INTO photo_metadata (
  photo_id,
  variant,
  file_size,
  mime_type
) VALUES (
  $1,
  $2,
  $3,
  $4
)
RETURNING *;
