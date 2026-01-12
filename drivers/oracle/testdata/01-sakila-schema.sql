-- Simplified Sakila schema for testing
-- This is a minimal version with just the actor table for cross-database testing

CREATE TABLE actor (
  actor_id SERIAL PRIMARY KEY,
  first_name VARCHAR(45) NOT NULL,
  last_name VARCHAR(45) NOT NULL,
  last_update TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert sample data
INSERT INTO actor (first_name, last_name, last_update) VALUES
  ('PENELOPE', 'GUINESS', '2006-02-15 04:34:33'),
  ('NICK', 'WAHLBERG', '2006-02-15 04:34:33'),
  ('ED', 'CHASE', '2006-02-15 04:34:33'),
  ('JENNIFER', 'DAVIS', '2006-02-15 04:34:33'),
  ('JOHNNY', 'LOLLOBRIGIDA', '2006-02-15 04:34:33'),
  ('BETTE', 'NICHOLSON', '2006-02-15 04:34:33'),
  ('GRACE', 'MOSTEL', '2006-02-15 04:34:33'),
  ('MATTHEW', 'JOHANSSON', '2006-02-15 04:34:33'),
  ('JOE', 'SWANK', '2006-02-15 04:34:33'),
  ('CHRISTIAN', 'GABLE', '2006-02-15 04:34:33');

-- Create an additional test table with various data types
CREATE TABLE film (
  film_id SERIAL PRIMARY KEY,
  title VARCHAR(255) NOT NULL,
  description TEXT,
  release_year INTEGER,
  rental_rate NUMERIC(4,2) NOT NULL DEFAULT 4.99,
  length SMALLINT,
  rating VARCHAR(10) DEFAULT 'G'
);

INSERT INTO film (title, description, release_year, rental_rate, length, rating) VALUES
  ('ACADEMY DINOSAUR', 'A Epic Drama of a Feminist And a Mad Scientist', 2006, 0.99, 86, 'PG'),
  ('ACE GOLDFINGER', 'A Astounding Epistle of a Database Administrator', 2006, 4.99, 48, 'G'),
  ('ADAPTATION HOLES', 'A Astounding Reflection of a Lumberjack', 2006, 2.99, 50, 'NC-17'),
  ('AFFAIR PREJUDICE', 'A Fanciful Documentary of a Frisbee', 2006, 2.99, 117, 'G'),
  ('AFRICAN EGG', 'A Fast-Paced Documentary of a Pastry Chef', 2006, 2.99, 130, 'G');
