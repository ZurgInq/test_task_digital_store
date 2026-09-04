CREATE TABLE issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id VARCHAR(255),
    sku VARCHAR(255),
    order_ext_id VARCHAR(255),
    code VARCHAR(255),
    status VARCHAR(255),
    err_reason VARCHAR(255),
    api_addr VARCHAR(4000),
    request_status VARCHAR(255),
    response_code INTEGER,
    
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);

CREATE UNIQUE INDEX issues_request_id ON issues(request_id);
CREATE INDEX issues_deleted_at ON issues(deleted_at);
CREATE INDEX issues_status ON issues(status);
CREATE INDEX issues_ext_id_resp_code ON issues(order_ext_id, response_code);
