-- name: GetAlbumPhoto :one
SELECT p.*, ap.album_id
FROM photos p
JOIN album_photos ap ON ap.photo_id = p.id
WHERE p.id = $1 LIMIT 1;

-- name: AddPhotoToAlbum :one
INSERT INTO album_photos (album_id, photo_id)
VALUES ($1, $2)
RETURNING *;

-- name: DeleteAlbumPhoto :one
DELETE FROM album_photos
WHERE album_id = $1 AND photo_id = $2
RETURNING id;

-- name: GetLastPhotoFromAlbum :one
SELECT * FROM album_photos
WHERE album_id = $1
ORDER BY created_at ASC
LIMIT 1;
