CREATE TABLE orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    group_id VARCHAR(255) NOT NULL,
    ext_id VARCHAR(255) NOT NULL,
    status VARCHAR(255) NOT NULL,
    product_id INTEGER NOT NULL,
    code VARCHAR(255) NOT NULL,
    amount INTEGER NOT NULL,
    
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);

CREATE INDEX orders_deleted_at ON orders(deleted_at);

CREATE UNIQUE INDEX orders_ext_id ON orders(ext_id);
CREATE INDEX orders_status ON orders(status);
CREATE INDEX orders_group_id ON orders(group_id);
