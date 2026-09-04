CREATE TABLE payments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id VARCHAR(255),
    order_id INTEGER,
    status VARCHAR(255),
    amount INTEGER,
    currency VARCHAR(255),
    
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);

CREATE INDEX payments_deleted_at ON payments(deleted_at);

CREATE UNIQUE INDEX payments_event_id ON payments(event_id);
CREATE INDEX payments_status ON payments(status);
CREATE INDEX payments_order_id ON payments(order_id);
