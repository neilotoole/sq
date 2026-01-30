-- Create test tables in Postgres for cross-database testing

CREATE TABLE IF NOT EXISTS customers (
    customer_id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO customers (name, email) VALUES
    ('John Doe', 'john@example.com'),
    ('Jane Smith', 'jane@example.com'),
    ('Bob Johnson', 'bob@example.com'),
    ('Alice Williams', 'alice@example.com'),
    ('Charlie Brown', 'charlie@example.com');

CREATE TABLE IF NOT EXISTS orders (
    order_id SERIAL PRIMARY KEY,
    customer_id INTEGER REFERENCES customers(customer_id),
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    total_amount DECIMAL(10,2)
);

INSERT INTO orders (customer_id, order_date, total_amount) VALUES
    (1, '2024-01-01 10:00:00', 99.99),
    (1, '2024-01-05 14:30:00', 149.99),
    (2, '2024-01-02 11:15:00', 79.99),
    (3, '2024-01-03 09:45:00', 199.99),
    (4, '2024-01-04 16:20:00', 59.99);
