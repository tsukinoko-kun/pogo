CREATE TABLE repositories (
  id   SERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);
CREATE INDEX repository_name ON repositories (name);

CREATE TABLE changes (
    id BIGSERIAL PRIMARY KEY,
    repository_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    author TEXT NOT NULL,
    device TEXT NOT NULL,
    depth BIGINT NOT NULL,
    FOREIGN KEY (repository_id) REFERENCES repositories (id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (repository_id, name)
);
CREATE INDEX change_name ON changes (name);

CREATE TABLE change_relations (
    change_id BIGINT NOT NULL,
    parent_id BIGINT,
    FOREIGN KEY (change_id) REFERENCES changes (id) ON DELETE CASCADE,
    FOREIGN KEY (parent_id) REFERENCES changes (id) ON DELETE CASCADE,
    UNIQUE (change_id, parent_id)
);
CREATE INDEX change_relation_change_id ON change_relations (change_id);

CREATE TABLE files (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    executable BOOLEAN NOT NULL,
    content_hash BYTEA NOT NULL,
    conflict BOOLEAN NOT NULL,
    UNIQUE (name, executable, content_hash)
);

CREATE TABLE change_files (
    change_id BIGINT NOT NULL,
    file_id BIGINT NOT NULL,
    FOREIGN KEY (change_id) REFERENCES changes (id) ON DELETE CASCADE,
    FOREIGN KEY (file_id) REFERENCES files (id) ON DELETE CASCADE,
    UNIQUE (change_id, file_id)
);
CREATE INDEX change_file_change_id ON change_files (change_id);


CREATE TABLE bookmarks (
    id BIGSERIAL PRIMARY KEY,
    repository_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    change_id BIGINT NOT NULL,
    FOREIGN KEY (repository_id) REFERENCES repositories (id) ON DELETE CASCADE,
    FOREIGN KEY (change_id) REFERENCES changes (id) ON DELETE CASCADE,
    UNIQUE (repository_id, name)
);
CREATE INDEX bookmark_name ON bookmarks (name);
