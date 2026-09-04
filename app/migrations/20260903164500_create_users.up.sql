CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    balance INTEGER,
    currency VARCHAR(255),
    
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);

CREATE INDEX users_deleted_at on users(deleted_at);
