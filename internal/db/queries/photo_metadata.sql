-- name: GetPhotoMetadataByPhotoID :many
SELECT * FROM photo_metadata
WHERE photo_id = $1;

-- name: CreatePhotoMetadata :one
INSERT INTO photo_metadata (
  photo_id,
  variant,
  width,
  height,
  file_size,
  mime_type
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6
)
RETURNING *;

-- name: GetPhotoMetadataByPhotoIDAndVariant :one
SELECT * FROM photo_metadata
where photo_id = $1 AND variant = $2;
