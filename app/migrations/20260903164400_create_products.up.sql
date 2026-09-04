CREATE TABLE products (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sku VARCHAR(255),
    name VARCHAR(255),
    type VARCHAR(255),
    price INTEGER,
    currency VARCHAR(255),
    image VARCHAR(255),
    
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);

CREATE INDEX products_deleted_at on products(deleted_at);
