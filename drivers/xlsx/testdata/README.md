This directory contains various Excel files for testing.

- `sakila.xlsx` is a full dump of Sakila.
- `sakila_subset.xlsx` is a subset of `sakila.xlsx` for faster testing. It
   contains only these sheets/tables: `actor`, `category`, `film`, `film_actor`, `language`.
- `sakila_noheader.xlsx` is the same as `sakila.xlsx`, but without the header
    row in each sheet.
- `test_invalid.xlsx` is a bad xlsx file that should fail to load.
- `test_header.xlsx` and `test_noheader.xlsx` exist to verify handling of
    table headers.
- `test_header_xlsx` is `test_header.xlsx` but without a file extension, to verify type detection.
- Various other files may exist to test specific issues.
