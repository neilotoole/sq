-- Create test tables with sample data for ClickHouse integration tests

-- Simple users table
CREATE TABLE IF NOT EXISTS users (id UInt64, username String, email String, created DateTime, is_active UInt8) ENGINE = MergeTree() ORDER BY id;

INSERT INTO users VALUES (1, 'alice', 'alice@example.com', '2024-01-01 10:00:00', 1), (2, 'bob', 'bob@example.com', '2024-01-02 11:00:00', 1), (3, 'charlie', 'charlie@example.com', '2024-01-03 12:00:00', 0), (4, 'diana', 'diana@example.com', '2024-01-04 13:00:00', 1), (5, 'eve', 'eve@example.com', '2024-01-05 14:00:00', 1);

-- Events table with partitioning
CREATE TABLE IF NOT EXISTS events (event_id UInt64, user_id UInt64, event_type String, event_time DateTime, properties String) ENGINE = MergeTree() PARTITION BY toYYYYMM(event_time) ORDER BY (user_id, event_time);

INSERT INTO events VALUES (1, 1, 'login', '2024-01-01 10:05:00', '{"ip":"192.168.1.1"}'), (2, 1, 'page_view', '2024-01-01 10:10:00', '{"page":"/home"}'), (3, 2, 'login', '2024-01-02 11:05:00', '{"ip":"192.168.1.2"}'), (4, 2, 'page_view', '2024-01-02 11:10:00', '{"page":"/profile"}'), (5, 3, 'login', '2024-01-03 12:05:00', '{"ip":"192.168.1.3"}');

-- Products table for type testing
CREATE TABLE IF NOT EXISTS products (product_id UInt32, name String, price Float64, in_stock UInt8, created_at DateTime, updated_at DateTime) ENGINE = MergeTree() ORDER BY product_id;

INSERT INTO products VALUES (1, 'Widget', 19.99, 1, '2024-01-01 00:00:00', '2024-01-01 00:00:00'), (2, 'Gadget', 29.99, 1, '2024-01-02 00:00:00', '2024-01-02 00:00:00'), (3, 'Doohickey', 39.99, 0, '2024-01-03 00:00:00', '2024-01-03 00:00:00');
