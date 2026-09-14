CREATE TABLE products (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sku VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(255) NOT NULL,
    price INTEGER NOT NULL,
    currency VARCHAR(255) NOT NULL,
    image VARCHAR(255) NOT NULL,
    
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);

CREATE INDEX products_deleted_at on products(deleted_at);
