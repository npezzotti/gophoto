-- name: ListPhotosByAlbum :many
SELECT p.*
FROM photos p
JOIN album_photos ap ON ap.photo_id = p.id
WHERE ap.album_id = $1
LIMIT $2
OFFSET $3;

-- name: GetAlbumPhoto :one
SELECT p.*, ap.album_id
FROM photos p
JOIN album_photos ap ON ap.photo_id = p.id
WHERE p.id = $1 LIMIT 1;

-- name: AddPhotoToAlbum :exec
INSERT INTO album_photos (album_id, photo_id)
VALUES ($1, $2);

-- name: RemovePhotoFromAlbum :exec
DELETE FROM album_photos
WHERE album_id = $1 AND photo_id = $2;
