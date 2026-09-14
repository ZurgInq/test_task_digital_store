CREATE TABLE payments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id VARCHAR(255),
    user_id INTEGER NOT NULL,
    order_group_id VARCHAR(255) NOT NULL,
    status VARCHAR(255) NOT NULL,
    amount INTEGER NOT NULL,
    currency VARCHAR(255) NOT NULL,
    
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);

CREATE INDEX payments_deleted_at ON payments(deleted_at);

CREATE UNIQUE INDEX payments_event_id ON payments(event_id);
CREATE INDEX payments_status ON payments(status);
CREATE INDEX payments_order_group_id ON payments(order_group_id);
