-- name: GetPhotoMetadataByPhotoID :many
SELECT * FROM photo_metadata
WHERE photo_id = $1;

-- name: CreatePhotoMetadata :one
INSERT INTO photo_metadata (
  photo_id,
  variant,
  file_size
) VALUES (
  $1,
  $2,
  $3
)
RETURNING *;
