-- name: getFileWithExecutable :one
SELECT id FROM files
WHERE content_hash = $1
    AND name = $2
    AND executable = $3
LIMIT 1;

-- name: getFileWithoutExecutable :one
SELECT id FROM files
WHERE content_hash = $1
    AND name = $2
LIMIT 1;

-- name: createFile :one
INSERT INTO files (name, executable, content_hash, conflict)
VALUES ($1, $2, $3, $4)
RETURNING id;
