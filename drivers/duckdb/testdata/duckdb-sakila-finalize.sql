-- Populate film_text from film. Run after sqlite-sakila-insert-data.sql (the
-- film rows must exist first). film_text is kept a PLAIN table: SQLite full-text
-- search (FTS5) requires a virtual/shadow table, which is schema-visible, so the
-- dedicated FTS fixture lives in sakila_fts5.db rather than here.
INSERT INTO film_text (film_id, title, description)
    SELECT film_id, title, description FROM film;
