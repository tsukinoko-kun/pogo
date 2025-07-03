-- name: GetRepoByName :one
SELECT id FROM repositories WHERE name = $1 LIMIT 1;

-- name: CreateRepo :one
INSERT INTO repositories (name)
VALUES ($1)
RETURNING id;

-- name: CreateChange :one
INSERT INTO changes (repository_id, name, description, author, device, depth)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: SetChangeParent :exec
INSERT INTO change_relations (change_id, parent_id)
VALUES ($1, $2)
ON CONFLICT (change_id, parent_id)
DO NOTHING;

-- name: GetAncestryOfChange :many
WITH RECURSIVE ancestry AS (
  -- Base case: the target change
  SELECT $1::bigint AS id
  UNION
  -- Recursive case: find all ancestors
  SELECT cr.parent_id
  FROM ancestry a
  JOIN change_relations cr
    ON cr.change_id = a.id
  WHERE cr.parent_id IS NOT NULL
)
SELECT
  c.*,
  cr.parent_id,
  cr.change_id
FROM ancestry a
JOIN changes c
  ON c.id = a.id
LEFT JOIN change_relations cr
  ON cr.change_id = c.id
WHERE c.repository_id = $3::integer
LIMIT $2;

-- name: SetChangeDescription :exec
UPDATE changes
SET description = $2, updated_at = CURRENT_TIMESTAMP, author = $3, device = $4
WHERE id = $1;

-- name: GetChangeDescription :one
SELECT description FROM changes WHERE id = $1 LIMIT 1;

-- name: GetChangeName :one
SELECT name FROM changes WHERE id = $1 AND repository_id = $2 LIMIT 1;

-- name: GetChangePrefix :one
WITH RECURSIVE lengths_series(l) AS (
  SELECT 1
  UNION ALL
  SELECT l + 1
  FROM lengths_series
  WHERE l < 64
)
SELECT
  (SUBSTRING(
     c.name
     FROM 1 FOR (
       SELECT ls.l
       FROM lengths_series AS ls
       WHERE ls.l <= CHAR_LENGTH(c.name)
         AND NOT EXISTS (
           SELECT 1
           FROM changes AS c_other
           WHERE c.id != c_other.id
             AND c.repository_id = c_other.repository_id
             AND SUBSTRING(c.name FROM 1 FOR ls.l)
               = SUBSTRING(c_other.name FROM 1 FOR ls.l)
         )
       ORDER BY ls.l
       LIMIT 1
     )
   )::TEXT) AS unique_identifier
FROM changes AS c
WHERE c.id = $1
  AND c.repository_id = $2
LIMIT 1;

-- name: ClearChange :exec
DELETE FROM change_files WHERE change_id = $1;

-- name: HasChangeChild :one
SELECT EXISTS (
    SELECT 1
    FROM change_relations
    WHERE parent_id = $1
);

-- name: GetChangeOwner :one
SELECT author, device FROM changes WHERE id = $1 LIMIT 1;

-- name: AddFileToChange :exec
INSERT INTO change_files (change_id, file_id)
VALUES ($1, $2);

-- name: CopyFileList :exec
INSERT INTO change_files (change_id, file_id)
SELECT @new_id, file_id
FROM change_files
AS old
WHERE old.change_id = @old_id;

-- name: CheckIfChangesSameFileCount :one
SELECT
    (SELECT COUNT(a.file_id) FROM change_files AS a WHERE a.change_id = $1) =
    (SELECT COUNT(b.file_id) FROM change_files AS b WHERE b.change_id = $2) AS have_same_number_of_files;

-- name: GetChangeIgnorefiles :many
SELECT files.name, files.content_hash FROM files
INNER JOIN change_files ON change_files.file_id = files.id
WHERE change_files.change_id = $1;

-- name: ListChangeFiles :many
SELECT files.name, files.executable, files.content_hash FROM change_files
INNER JOIN files ON files.id = change_files.file_id
WHERE change_files.change_id = $1;

-- name: SetBookmark :exec
INSERT INTO bookmarks (repository_id, name, change_id)
VALUES ($1, $2, $3)
ON CONFLICT (repository_id, name)
DO UPDATE SET change_id = $3;

-- name: GetBookmark :one
SELECT change_id FROM bookmarks WHERE repository_id = $1 AND name = $2 LIMIT 1;

-- name: findChanges :many
SELECT DISTINCT c.id
FROM changes AS c
LEFT JOIN bookmarks AS b
    ON b.change_id = c.id AND b.repository_id = c.repository_id
WHERE c.repository_id = sqlc.arg('repository_id')
    AND (c.name LIKE sqlc.arg('search')::text || '%' OR b.name = sqlc.arg('search')::text)
LIMIT sqlc.arg('limit');

-- name: FindChangeExact :one
SELECT * FROM changes WHERE repository_id = $1 AND name = $2 LIMIT 1;

-- name: GetChangeDepth :one
SELECT depth FROM changes WHERE id = $1 AND repository_id = $2 LIMIT 1;

-- name: SetChangeDepth :exec
UPDATE changes
SET depth = $2
WHERE id = $1;

-- name: GetAllBookmarks :many
SELECT * FROM bookmarks
WHERE repository_id = $1 
    AND bookmarks.name NOT LIKE '__head-%';

-- name: HasChangeConflicts :one
SELECT EXISTS (
    SELECT 1
    FROM change_files
    INNER JOIN files ON files.id = change_files.file_id
    WHERE change_files.change_id = $1 AND files.conflict = true
    LIMIT 1
);

-- name: GetChangeConflicts :many
SELECT files.name
FROM change_files
INNER JOIN files ON files.id = change_files.file_id
WHERE change_files.change_id = $1 AND files.conflict = true;
