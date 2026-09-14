CREATE TABLE balance_transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    order_id INTEGER,
    payment_id INTEGER,
    amount INTEGER NOT NULL,
    operation VARCHAR(255) NOT NULL,
    
    created_at DATETIME NOT NULL
);

CREATE INDEX bt_user_op_amount on balance_transactions(user_id, operation, amount);
CREATE INDEX bt_payment_id on balance_transactions(payment_id);
CREATE INDEX bt_order_id on balance_transactions(order_id);
CREATE UNIQUE INDEX bt_user_order_op on balance_transactions(user_id, order_id, operation);
CREATE UNIQUE INDEX bt_user_payment_op on balance_transactions(user_id, payment_id, operation);
