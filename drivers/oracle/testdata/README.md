# Test Data

This directory contains SQL initialization scripts for the Postgres test database.

## Files

### 01-sakila-schema.sql

Simplified Sakila dataset for cross-database integration testing. Includes:

- **actor table**: 10 sample actors for testing data migration
- **film table**: 5 sample films demonstrating various data types

This schema is automatically loaded when the Postgres container starts via docker-compose.

## Usage

The scripts in this directory are automatically executed by the Postgres container on first startup (via the `/docker-entrypoint-initdb.d` mount in `../testutils/docker-compose.yml`).

Files are executed in alphanumeric order, so prefix with numbers to control execution order:
- `01-sakila-schema.sql` - Schema and initial data
- `02-additional-data.sql` - Additional test data (if needed)

## Full Sakila Dataset

This is a simplified version of the Sakila dataset. The full dataset includes:
- 16 tables (actor, film, customer, rental, payment, etc.)
- Relationships demonstrating foreign keys
- Views and stored procedures

For testing purposes, we use only the actor and film tables to verify:
- Cross-database connectivity
- Data type mapping (Int, Text, Decimal, Timestamp)
- Row-level data integrity
- Table creation and insertion

The full Sakila dataset is available at:
- PostgreSQL: https://github.com/jOOQ/sakila
- MySQL: https://dev.mysql.com/doc/sakila/en/
