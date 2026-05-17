// =============================================================================
// SLQ.g4 -- the ANTLR4 grammar for SLQ, the sq query language.
// =============================================================================
//
// SLQ is the query language used by `sq` (https://sq.io). It is a
// pipeline-oriented language strongly inspired by `jq`, but with semantics
// that map cleanly onto SQL. A simple SLQ query looks like this:
//
//     @sakila | .actor | .first_name, .last_name | .[0:10]
//
// which, when run against a Postgres source, is rendered as approximately:
//
//     SELECT "first_name", "last_name" FROM "actor" LIMIT 10 OFFSET 0
//
// The grammar is NOT stable: it is subject to change in any new sq release.
// Companion documentation lives in `README.md` next to this file, and the
// `testdata/` directory has a corpus of sample queries that exercise the
// grammar.
//
// -----------------------------------------------------------------------------
// Language tour (for grammar readers)
// -----------------------------------------------------------------------------
//
// A query is a sequence of *segments* separated by the pipe operator `|`.
// Each segment contains one or more *elements* separated by commas.
//
//     @sakila    |    .actor    |    .first_name, .last_name    |    .[0:10]
//     └────────┘    └──────┘    └─────────────────────────┘    └─────┘
//      handle      table-sel      column selectors             row-range
//      segment     segment        segment                       segment
//
// Most segments map directly to a clause of the eventual SQL statement:
//
//     element kind         purpose                            SQL analogue
//     ─────────────────    ────────────────────────────────   ──────────────
//     handle               choose the data source             (source pick)
//     handleTable          handle + table in one token        FROM <table>
//     selectorElement      pick a table or column             FROM / SELECT
//     join                 join two tables                    JOIN ... ON ...
//     where                filter predicate                   WHERE
//     groupBy              grouping                           GROUP BY
//     having               group filter                       HAVING
//     orderBy              sort                               ORDER BY
//     rowRange             limit/offset                       LIMIT / OFFSET
//     uniqueFunc           dedupe                             DISTINCT
//     countFunc            count rows                         COUNT(...)
//     funcElement          generic function call              <fn>(...)
//     exprElement          expression as a result column      <expr> AS <a>
//
// -----------------------------------------------------------------------------
// Where this file fits
// -----------------------------------------------------------------------------
//
// 1. `antlr4` (Java) reads this `.g4` file and generates Go sources for the
//    lexer, parser, listener, and visitor. Generation is wired through
//    `generate.sh` / `generate.go` (`go generate ./...`). The output lands
//    in `libsq/ast/internal/slq/`; those files MUST NOT be hand-edited.
//
// 2. `libsq/ast.Parse` is the public entry point. It wraps the generated
//    lexer/parser, walks the resulting parse tree with `parseTreeVisitor`,
//    and produces an `*ast.AST` (in package `libsq/ast`).
//
// 3. A SQL renderer (`libsq/ast/render`) then walks the AST and emits a
//    dialect-specific SQL statement.
//
// See `README.md` for diagrams of these stages.
// =============================================================================

grammar SLQ;

// -----------------------------------------------------------------------------
// PARSER RULES
// -----------------------------------------------------------------------------
//
// ANTLR convention: rule names that start with a lower-case letter are
// PARSER rules; ALL_CAPS names are LEXER (token) rules. The parser rules
// below define the tree structure; the lexer rules at the bottom of the
// file define the alphabet of tokens that the parser sees.

// stmtList is the top-level entry point: zero or more queries separated by
// semicolons. Leading and trailing semicolons are tolerated, and runs of
// consecutive semicolons are collapsed. This permits scripts like:
//
//     ; @sakila | .actor ;; @sakila | .film ;
//
// although in practice nearly all SLQ inputs are a single query and the
// semicolon machinery is rarely used. `libsq/ast` builds an AST per query.
stmtList: ';'* query ( ';'+ query)* ';'*;

// query is a sequence of segments joined by the pipe operator. The pipe
// is the defining feature of SLQ: each segment receives "what the previous
// segment produced" and transforms it further, mirroring `jq` and Unix
// shell pipelines. The pipe has no associativity issue because segments
// are simply concatenated left-to-right.
//
//     @sakila | .actor | .first_name
//     └──┬──┘   └──┬──┘   └────┬───┘
//       seg0      seg1        seg2
query: segment ('|' segment)*;

// segment is a comma-separated list of elements. Most segments contain a
// single element (e.g. a `where(...)` or an `order_by(...)`). The comma
// is mainly used for projections — `.first_name, .last_name` — and for
// listing tables before a join: `.actor, .film | join(...)`.
segment: (element) (',' element)*;

// element is the heart of the parser: each alternative names one kind of
// thing that can appear inside a segment. Order matters here only when
// alternatives would otherwise be ambiguous; ANTLR uses the first match.
//
// Note that `handleTable` MUST come before `handle`: an input like
// `@sakila.actor` is a `handleTable` (handle + table), but the prefix
// `@sakila` on its own is a `handle`. Listing `handleTable` first ensures
// the longer match wins.
element
  : handleTable
	| handle
	| selectorElement
	| join
	| groupBy
	| having
	| orderBy
	| rowRange
	| uniqueFunc
	| countFunc
	| where
	| funcElement
	| exprElement;



// -----------------------------------------------------------------------------
// Function calls (generic)
// -----------------------------------------------------------------------------

// funcElement wraps a function call so that it can be used as a result
// column with an optional alias:
//
//     .payment | sum(.amount):total
//
// `sum(.amount)` is the `func`; `:total` is the `alias`.
funcElement: func (alias)?;

// func is the syntactic form `name(arg, arg, ...)`. The single-argument
// form `name(*)` is also accepted (e.g. `count(*)`). Empty parens are
// permitted: `name()`.
func: funcName '(' ( expr ( ',' expr)* | '*')? ')';

// funcName is the closed set of "portable" function names known to SLQ,
// plus the open-ended `PROPRIETARY_FUNC_NAME` for DB-native escapes.
// Adding a new portable function requires (a) adding the literal here and
// (b) implementing/handling it in `libsq/ast` and the SQL renderer.
funcName
  : 'sum'
	| 'avg'
	| 'max'
	| 'min'
	| 'schema'
	| 'catalog'
	| 'rownum'
	| 'contains'
	| 'startswith'
	| 'endswith'
	| 'icontains'
	| 'istartswith'
	| 'iendswith'
	| 'like'
	| 'ilike'
	| PROPRIETARY_FUNC_NAME
  ;

// PROPRIETARY_FUNC_NAME is the escape hatch for invoking a function that
// is specific to a particular database, e.g. SQLite's `strftime`. The
// convention is to prefix the underscore: `_strftime(...)`. The renderer
// passes proprietary calls through verbatim (minus the leading `_`) and
// it is the user's responsibility to ensure the function exists on the
// active source. See `ast.FuncNode.IsProprietary`.
PROPRIETARY_FUNC_NAME: '_' ID;


// -----------------------------------------------------------------------------
// join
// -----------------------------------------------------------------------------
//
// `join` implements SQL's JOIN. The shape is `<join-kind>(table, predicate)`,
// optionally with cross-source handles and aliases.
//
//     @sakila_pg | .actor | join(.film_actor, .actor_id)
//     @sakila_pg | .actor | join(@sakila_my.film_actor, .actor_id)
//     @sakila_pg | .actor | join(.film_actor, .actor.actor_id == .film_actor.actor_id)
//     @sakila_pg | .actor:a | join(.film_actor:fa, .a.actor_id == .fa.actor_id)
//     @sakila_pg.actor:a | join(@sakila_my.film_actor:fa, .a.actor_id == .fa.actor_id)
//
// The second-argument expression may be omitted for the few join types
// (e.g. `cross_join`) that don't require a predicate.
//
// See:
// - https://www.sqlite.org/syntax/join-clause.html
// - https://www.sqlite.org/syntax/join-operator.html
join: JOIN_TYPE '(' joinTable (',' expr)? ')';

// joinTable is the table being joined to. The optional leading `HANDLE`
// supports cross-source joins (e.g. `@sakila_my.film_actor`). The trailing
// `alias` provides a short name for use in the join predicate.
joinTable: (HANDLE)? NAME (alias)?;

// JOIN_TYPE is the closed set of join keywords plus short aliases.
// `NATURAL JOIN` is intentionally absent: its implementation differs
// across DBs and it is widely considered an anti-pattern. Not every
// listed kind is supported by every backing database, but that is the
// renderer/driver's problem to detect, not the grammar's.
JOIN_TYPE
 : 'join'
 | 'inner_join'
 | 'left_join'
 | 'ljoin'
 | 'left_outer_join'
 | 'lojoin'
 | 'right_join'
 | 'rjoin'
 | 'right_outer_join'
 | 'rojoin'
 | 'full_outer_join'
 | 'fojoin'
 | 'cross_join'
 | 'xjoin'
 ;


// -----------------------------------------------------------------------------
// uniqueFunc -- DISTINCT
// -----------------------------------------------------------------------------
//
// `unique` (or its short alias `uniq`) implements SQL's `DISTINCT`. It
// takes zero arguments and applies to the current projection:
//
//     .actor | .first_name | unique     -- SELECT DISTINCT first_name FROM actor
//     .actor | unique                   -- SELECT DISTINCT * FROM actor
//
// Note that `unique` is a *segment-level* element (sibling to `where`,
// `group_by`, etc.), not a function call: there are no parentheses.
uniqueFunc: 'unique' | 'uniq';


// -----------------------------------------------------------------------------
// countFunc -- COUNT
// -----------------------------------------------------------------------------
//
// `count` has bespoke grammar because it accepts several distinct shapes,
// some of which would conflict with the generic `func` rule:
//
//     .actor | count                          -- bare keyword form
//     .actor | count:quantity                 -- bare form with alias
//     .actor | count()                        -- empty-paren form
//     .actor | count(*)                       -- COUNT(*)
//     .actor | count(.first_name):quantity    -- COUNT(<col>) with alias
//
// In particular, the bare form `count` (no parens) is the reason this
// can't just live under `funcName`/`func`.
countFunc: 'count' (LPAR (selector)? RPAR)? (alias)?;


// -----------------------------------------------------------------------------
// where -- WHERE
// -----------------------------------------------------------------------------
//
// `where` filters rows, exactly like SQL's `WHERE` clause:
//
//     .actor | where(.actor_id > 10 && .first_name == "TOM")
//
// Naming note: this construct is called both `where` (the SQL term) and
// `select` (the `jq` term -- see `jq`'s `select(boolean_expression)`).
// `sq`'s design principle is to mirror `jq` syntax where possible, but
// for SQL audiences "select" means "pick these columns", which conflicts.
// For now BOTH spellings are accepted; one is expected to be deprecated
// after user feedback settles the question.

// WHERE is the lexer rule that captures whichever spelling the user used.
// The parser rule below sees a single `WHERE` token either way.
WHERE: 'where' | 'select';
where: WHERE LPAR (expr)? RPAR;


// -----------------------------------------------------------------------------
// group_by -- GROUP BY
// -----------------------------------------------------------------------------
//
// `group_by` implements SQL `GROUP BY`. It is typically paired with
// aggregate functions in the preceding projection:
//
//     .payment | .customer_id, sum(.amount) | group_by(.customer_id)
//
// Spellings:
// - `group_by` (the `jq` form; see
//   https://jqlang.github.io/jq/manual/v1.6/#group_by(path_expression))
// - `gb`        (brevity alias)

GROUP_BY: 'group_by' | 'gb';

// groupByTerm is what may appear inside `group_by(...)`: either a column
// selector or a function call (so that you can group by an expression).
groupByTerm: selector | func;

groupBy: GROUP_BY '(' groupByTerm (',' groupByTerm)* ')';


// -----------------------------------------------------------------------------
// having -- HAVING
// -----------------------------------------------------------------------------
//
// `having` filters grouped results -- SQL's `HAVING`. It is a top-level
// segment element, and (per SQL semantics) must follow a `group_by`:
//
//     .payment | .customer_id, sum(.amount) |
//       group_by(.customer_id) | having(sum(.amount) > 100)
//
// The grammar does not enforce the "must follow group_by" rule -- that
// constraint is checked by AST verification in `libsq/ast`.

HAVING: 'having';
having: HAVING '(' expr ')';


// -----------------------------------------------------------------------------
// order_by -- ORDER BY
// -----------------------------------------------------------------------------
//
// `order_by` implements SQL `ORDER BY`. Trailing `+` and `-` on a term
// select ASC / DESC respectively (default is ASC).
//
//     .actor | order_by(.first_name, .last_name)
//     .actor | order_by(.first_name+)
//     .actor | order_by(.actor.first_name-)
//
// Spellings:
// - `order_by` (canonical)
// - `sort_by`  (the `jq` form; see
//   https://jqlang.github.io/jq/manual/v1.6/#sort,sort_by(path_expression))
// - `ob`       (brevity alias)
//
// Note: there is NO `sort` synonym for `jq`'s `sort`. SQL results are
// inherently ordered when an ORDER BY is present, so a bare `sort` with
// no key has no obvious mapping. It could conceivably be added as a
// no-op, but that has not been done.

ORDER_BY: 'order_by' | 'sort_by' | 'ob';

// orderByTerm is a selector with an optional direction suffix.
orderByTerm: selector ('+' | '-')?;

orderBy: ORDER_BY '(' orderByTerm (',' orderByTerm)* ')';


// -----------------------------------------------------------------------------
// Selectors -- pick a table/column
// -----------------------------------------------------------------------------
//
// A "selector" is a dotted path that names either a table, a column, or
// a table-qualified column:
//
//     .first_name              -- column
//     ."first name"            -- column (quoted, allows spaces/keywords)
//     .actor                   -- table
//     ."actor"                 -- table (quoted)
//     .actor.first_name        -- table-qualified column
//
// The grammar permits ONE or TWO name parts. Whether a single-part
// selector resolves to a column or a table is decided by `libsq/ast`
// based on position in the query (see `narrowTblSel`, `narrowColSel`,
// `narrowTblColSel` in `libsq/ast`).
selector: NAME (NAME)?;

// selectorElement wraps a selector so it can carry an alias when used as
// a projection result column:
//
//     .first_name:given_name
//     ."first name":given_name
//     .actor.first_name:given_name
//     ."actor".first_name
//
// `:given_name` is the alias part (see `alias` rule).
selectorElement: (selector) (alias)?;


// -----------------------------------------------------------------------------
// Aliases
// -----------------------------------------------------------------------------
//
// An alias attaches a name to a column/expression/table -- the SQL `AS`.
// The canonical form is a colon followed by an identifier, argument, or
// quoted string:
//
//     .first_name:given_name        -- alias = "given_name"
//     .actor:a                      -- table alias
//     (1+2):total                   -- expression alias
//     .first_name:"given name"      -- alias with spaces
//
// `ALIAS_RESERVED` is a wart: see comment on that lexer rule below.
alias: ALIAS_RESERVED | ':' (ARG | ID | STRING);

// ALIAS_RESERVED works around an ANTLR pain point: when an alias text
// happens to be a reserved keyword (e.g. `:count` after a `count`
// function), the lexer would otherwise tokenize the keyword and the
// parser would get confused. Each entry here is a literal `:keyword`
// pre-baked as a single token, sidestepping the conflict.
//
// This list must be kept in sync as new keywords are added. The grammar
// could likely be refactored to eliminate the need for this, but that
// has not been done.
ALIAS_RESERVED
    // TODO: Update ALIAS_RESERVED with all "keywords"
    : ':count'
    | ':count_unique'
    | ':avg'
    | ':group_by'
    | ':max'
    | ':min'
    | ':order_by'
    | ':unique'
    ;

// ARG is a `$`-prefixed argument reference. Arguments are bound from
// outside the query (e.g. CLI flags) and substituted at evaluation time.
ARG: '$' ID;

arg : ARG;


// -----------------------------------------------------------------------------
// Source handles & handle-table pairs
// -----------------------------------------------------------------------------
//
// A "handle" identifies a data source registered in `sq`'s configuration.
// Handles begin with `@` and may use `/` separators for hierarchical
// organization:
//
//     @sakila
//     @work/acme/sakila
//     @home/csv/actor
//
// A "handle-table" jams a handle and a table into one element, allowing
// the source segment to be elided:
//
//     @sakila.actor             -- equivalent to: @sakila | .actor
//     @sakila.actor:a           -- with alias on the table (in handleTable's
//                                   parent context, not in the lexer itself)

// handleTable is `@source.table`.
handleTable: HANDLE NAME;

// handle is a bare source handle: `@source`.
handle: HANDLE;


// -----------------------------------------------------------------------------
// rowRange -- LIMIT / OFFSET
// -----------------------------------------------------------------------------
//
// Row ranges use `jq`-style bracket slicing. The forms are:
//
//     .[]         no range -- select all rows (no LIMIT, no OFFSET)
//     .[10]       single row at index 10 -- LIMIT 1 OFFSET 10
//     .[10:15]    rows 10 through 15 -- LIMIT (15-10) OFFSET 10
//     .[0:15]     first 15 rows -- LIMIT 15 OFFSET 0
//     .[:15]      same as .[0:15]
//     .[10:]      from row 10 onwards -- OFFSET 10, no LIMIT
//
// The translation from slice form to {offset, limit} happens in
// `libsq/ast.VisitRowRange`.
rowRange:
	'.[' (
		NN COLON NN // [10:15]
		| NN COLON // [10:]
		| COLON NN // [:15]
		| NN // [10]
	)? ']';


// -----------------------------------------------------------------------------
// Expressions
// -----------------------------------------------------------------------------
//
// exprElement is an expression used as a result column (with optional
// alias):
//
//     .actor | (1+2):total
//
// See `ast.ExprElementNode`.
exprElement: expr (alias)?;

// expr is the expression grammar. ANTLR resolves operator precedence by
// the *order* in which alternatives are listed: earlier rules bind
// tighter. The order below matches SQL/C-family precedence, from
// tightest to loosest:
//
//     1.  '(' expr ')'         -- parenthesized
//     2.  selector             -- column reference
//     3.  literal              -- number / string / bool / null
//     4.  arg                  -- $param reference
//     5.  unary  -x  +x  ~x  !x
//     6.  ||                   -- string concat (NOT logical-or)
//     7.  *  /  %
//     8.  +  -
//     9.  <<  >>  &            -- bitwise
//     10. <  <=  >  >=
//     11. ==  !=
//     12. &&                   -- logical-and (and logical-or sibling)
//     13. func                 -- function call
//
// IMPORTANT: `||` here is the SQL string-concatenation operator (as in
// SQLite/Postgres `'a' || 'b' = 'ab'`), NOT logical-or. Logical-or, if
// needed, is generally expressed via separate `where` calls or rewritten
// by callers; the grammar tier does not currently introduce a dedicated
// logical-or token.
expr:
	'(' expr ')'
	| selector
	| literal
	| arg
	| unaryOperator expr
	| expr '||' expr
	| expr ( '*' | '/' | '%') expr
	| expr ( '+' | '-') expr
	| expr ( '<<' | '>>' | '&') expr
	| expr ( '<' | '<=' | '>' | '>=') expr
	| expr ( '==' | '!=' |) expr
	| expr '&&' expr
	| func
	;

// literal is the set of scalar literals admitted by an expression.
literal: NN | NUMBER | BOOL | STRING | NULL;

// unaryOperator -- standard set, including bitwise-not (`~`) and
// logical-not (`!`).
unaryOperator: '-' | '+' | '~' | '!';


// =============================================================================
// LEXER RULES
// =============================================================================
//
// Lexer rules (ALL_CAPS) match raw character sequences and produce tokens
// for the parser. Order MATTERS when two rules can match the same input:
// ANTLR prefers (1) the longer match, and (2) on a tie, the rule defined
// earlier. Several rules below comment on that ordering explicitly.
// =============================================================================

// BOOL captures `true`/`false`. Note that this matches before `ID`
// because lexer rules are tried in definition order on equal-length
// matches and `ID` is defined later.
BOOL: 'true' | 'false';

// NULL captures `null`. Same ordering note as BOOL.
NULL: 'null';

// ID -- standard identifier. Letters, digits, underscores; cannot start
// with a digit. Examples: `actor`, `first_name`, `_private`.
ID: [a-zA-Z_][a-zA-Z0-9_]*;

// IDNUM matches numeric-prefixed identifiers: digits followed by at
// least one letter or underscore. Examples: `123abc`, `007bond`,
// `456_schema`. Added for issue #470 (numeric schema/catalog names).
IDNUM: [0-9]+ [a-zA-Z_] [a-zA-Z0-9_]*;

// WS -- whitespace is skipped entirely; SLQ is not whitespace-sensitive
// (except inside quoted strings).
WS: [ \t\r\n]+ -> skip;

// Punctuation tokens. Naming these explicitly lets parser rules
// reference them by name in addition to literal form, and helps when
// inspecting generated parse trees.
LPAR: '(';
RPAR: ')';
LBRA: '[';
RBRA: ']';
COMMA: ',';
PIPE: '|';
COLON: ':';


// NN -- natural number {0, 1, 2, ...}. Used for row-range indices and
// other non-negative-integer-only contexts. Defined BEFORE `DIGITS` so
// that on equal-length matches `NN` wins for plain integers without
// leading zeros.
NN: INTF;

// NUMBER -- signed numeric literal, possibly fractional and possibly
// with exponent. Examples:
//
//     45         1e10       -3e4
//     -3         1.35       -4.5
//     0.3        1.35E-9
NUMBER:
	NN
	| '-'? INTF '.' [0-9]+ EXP? // 1.35, 1.35E-9, 0.3, -4.5
	| '-'? INTF EXP // 1e10 -3e4
	| '-'? INTF ; // -3, 45

// INTF is a fragment used by NN and NUMBER: an integer literal with no
// leading zeros. (`fragment` means it does not produce a token by
// itself; it only contributes to other rules.)
fragment INTF: '0' | [1-9] [0-9]*;

// DIGITS matches pure digit sequences INCLUDING those with leading
// zeros. Examples: `007`, `00123`, `42`. Used inside the `NAME` rule
// for numeric schema/catalog names (issue #470). This is separate from
// `INTF`, which disallows leading zeros so that numeric literals don't
// accidentally accept `007` and the like.
//
// IMPORTANT: `DIGITS` must be defined AFTER `NN` so that for inputs
// without leading zeros (e.g. `42`) the lexer picks `NN`. ANTLR breaks
// equal-length ties by definition order, earliest wins.
DIGITS: [0-9]+;

// EXP is the exponent fragment used by NUMBER. `\-` inside the bracket
// class is needed because `-` denotes a range otherwise.
fragment EXP:
	[Ee] [+\-]? INTF;

// Comparison operators. Defined as named tokens so parser rules can
// reference them symbolically. Note that the parser rule `expr` above
// happens to use the literal forms (`'<'`, `'=='`, etc.) directly; both
// styles are interchangeable in ANTLR.
LT_EQ: '<=';
LT: '<';
GT_EQ: '>=';
GT: '>';
NEQ: '!=';
EQ: '==';


// NAME matches a leading dot plus an identifier-like payload, modeling
// the `.thing` syntax used throughout SLQ. Alternatives, in priority
// order (ANTLR's maximal munch chooses the longest match):
//
//   - ARG:    argument reference, e.g. `.$var`
//   - ID:     standard identifier, e.g. `.actor`, `._private`
//   - STRING: quoted identifier, e.g. `."my table"`
//   - DIGITS: pure digit sequence including leading zeros, e.g. `.007`
//             (issue #470 -- numeric schema/catalog names)
//   - IDNUM:  numeric-prefixed identifier, e.g. `.123abc`, `.007bond`
//             (issue #470)
//
// Maximal-munch matters here: for input `.007bond`, the lexer must
// produce a single `NAME` whose payload matches `IDNUM` (`007bond`),
// NOT a `NAME` containing only `DIGITS` (`007`) followed by stray
// `bond`. ANTLR's longest-match rule handles this automatically.
NAME: '.' (ARG | ID | STRING | DIGITS | IDNUM);

// (Old SEL idea kept for reference -- a fully-dotted multi-segment
// selector matched in the lexer rather than the parser. Not used today
// because two-part selectors are handled in the parser `selector` rule
// instead.)
//SEL: '.' (ID | STRING) ('.' (ID | STRING))*;


// HANDLE matches a source handle: `@` plus identifier, optionally with
// `/`-separated path segments. Examples:
//
//     @mydb1
//     @postgres_db2
//     @work/acme/sakila
HANDLE: '@' ID ('/' ID)*;

// STRING is a JSON-style double-quoted string with backslash escapes.
// SLQ uses `STRING` both for string LITERALS in expressions and for
// QUOTED IDENTIFIERS inside `NAME` (e.g. `."my table"`). The AST layer
// strips the surrounding quotes (`stringz.StripDoubleQuote`).
STRING: '"' (ESC | ~["\\])* '"';

// Escape sequences supported inside STRING: standard JSON-style plus a
// `\uXXXX` Unicode escape.
fragment ESC: '\\' (["\\/bfnrt] | UNICODE);
fragment UNICODE: 'u' HEX HEX HEX HEX;
fragment HEX: [0-9a-fA-F];

//NUMERIC_LITERAL
// : DIGIT+ ( '.' DIGIT* )? ( E [-+]? DIGIT+ )? | '.' DIGIT+ ( E [-+]? DIGIT+ )? ;

// DIGIT -- single decimal digit fragment, used by various number rules.
fragment DIGIT: [0-9];

// Case-insensitive letter fragments. These are kept around for use by
// any future rule that wants to match keywords case-insensitively
// (`A B C ...` -> `[aA][bB][cC]...`). They are not currently referenced,
// but are preserved both for convenience and because they appear in the
// grammar's upstream lineage (see the SQLite grammar reference at the
// bottom of this file).
fragment A: [aA];
fragment B: [bB];
fragment C: [cC];
fragment D: [dD];
fragment E: [eE];
fragment F: [fF];
fragment G: [gG];
fragment H: [hH];
fragment I: [iI];
fragment J: [jJ];
fragment K: [kK];
fragment L: [lL];
fragment M: [mM];
fragment N: [nN];
fragment O: [oO];
fragment P: [pP];
fragment Q: [qQ];
fragment R: [rR];
fragment S: [sS];
fragment T: [tT];
fragment U: [uU];
fragment V: [vV];
fragment W: [wW];
fragment X: [xX];
fragment Y: [yY];
fragment Z: [zZ];

// LINECOMMENT -- `#` to end-of-line. Skipped, like whitespace.
LINECOMMENT: '#' .*? '\n' -> skip;

//// From https://github.com/antlr/grammars-v4/blob/master/sql/sqlite/SQLiteLexer.g4
//IDENTIFIER:
//    '"' (~'"' | '""')* '"'
//    | '`' (~'`' | '``')* '`'
//    | '[' ~']'* ']'
//    | [A-Z_] [A-Z_0-9]*
//; // TODO check: needs more chars in set
