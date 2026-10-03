-- Each machine's Access service token (its Client ID) and the one device it posts for. Written by
-- mise run access:token -- create, deleted by -- revoke.
CREATE TABLE IF NOT EXISTS machines (token TEXT PRIMARY KEY, device TEXT NOT NULL, name TEXT NOT NULL, created INTEGER NOT NULL);
