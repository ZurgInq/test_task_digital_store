CREATE TABLE orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    ext_id VARCHAR(255),
    status VARCHAR(255),
    product_ids TEXT,
    code VARCHAR(255) NOT NULL,
    
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);

CREATE INDEX orders_deleted_at ON orders(deleted_at);

CREATE UNIQUE INDEX orders_ext_id ON orders(ext_id);
CREATE INDEX orders_status ON orders(status);
