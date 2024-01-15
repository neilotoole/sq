// This is the grammar for SLQ, the query language used by sq (https://sq.io).
// The grammar is not yet finalized; it is subject to change in any new sq release.
grammar SLQ;

stmtList: ';'* query ( ';'+ query)* ';'*;

query: segment ('|' segment)*;

segment: (element) (',' element)*;

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



funcElement: func (alias)?;
func: funcName '(' ( expr ( ',' expr)* | '*')? ')';
funcName
  : 'sum'
	| 'avg'
	| 'max'
	| 'min'
	| 'schema'
	| 'catalog'
	| 'rownum'
	| PROPRIETARY_FUNC_NAME
  ;

// PROPRIETARY_FUNC_NAME is a DB-native func, which is invoked by prefixing
// an underscore to the func name, e.g. _date(xyz).
PROPRIETARY_FUNC_NAME: '_' ID;

/*
join
----

join implements SQL's JOIN mechanism.

    @sakila_pg | .actor | join(.film_actor, .actor_id)
    @sakila_pg | .actor | join(@sakila_my.film_actor, .actor_id)
    @sakila_pg | .actor | join(.film_actor, .actor.actor_id == .film_actor.actor_id)
    @sakila_pg | .actor:a | join(.film_actor:fa, .a.actor_id == .fa.actor_id)
    @sakila_pg.actor:a | join(@sakila_my.film_actor:fa, .a.actor_id == .fa.actor_id)

See:
- https://www.sqlite.org/syntax/join-clause.html
- https://www.sqlite.org/syntax/join-operator.html
*/
join: JOIN_TYPE '(' joinTable (',' expr)? ')';
joinTable: (HANDLE)? NAME (alias)?;
// JOIN_TYPE is the set of join types, and their aliases.
// Note that not every database may support every join type, but
// this is not the concern of the grammar.
//
// Note that NATURAL JOIN is not supported, as its implementation
// is spotty in various DBs, and it's often considered an anti-pattern.
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


/*
uniqueFunc
----------

uniqueFunc implements SQL's DISTINCT mechanism.

    .actor | .first_name | unique
    .actor | unique

The func takes zero args.
*/
uniqueFunc: 'unique' | 'uniq';

/*

countFunc
---------

This implements SQL's COUNT function. It has special handling vs other
funcs because of the several forms it can take.

    .actor | count
    .actor | count:quantity                 # alias
    .actor | count()
    .actor | count(*)
    .actor | count(.first_name):quanity     # alias

 */
countFunc: 'count' (LPAR (selector)? RPAR)? (alias)?;


/*
where
-----
The "where" mechanism implements SQL's WHERE clause.

  .actor | where(.actor_id > 10 && .first_name == "TOM")

From a SQL perspective, "where" is the natural name to use. However,
and inconveniently, jq uses "select" for this purpose.

 https://jqlang.github.io/jq/manual/v1.6/#select(boolean_expression)

One of sq's design principles is to adhere to jq syntax whenver possible.
Alas, for SQL users, "select" means "SELECT these columns", not "select
the matching rows".

It's unclear what the best approach is here, so for now, we will allow
both "where" and "select", and plan to deprecate one of these after user
feedback.
*/

WHERE: 'where' | 'select';
where: WHERE LPAR (expr)? RPAR;


/*
group_by
--------

The 'group_by' construct implments the SQL "GROUP BY" clause.

    .payment | .customer_id, sum(.amount) | group_by(.customer_id)

Syonyms:
- 'group_by' for jq interoperability.
  https://jqlang.github.io/jq/manual/v1.6/#group_by(path_expression)
- 'gb' for brevity.
*/

GROUP_BY: 'group_by' | 'gb';
groupByTerm: selector | func;
groupBy: GROUP_BY '(' groupByTerm (',' groupByTerm)* ')';


/*
having
------

The 'having' construct implements the SQL "HAVING" clause.
It is a top-level segment clause, and must be preceded by a 'group_by' clause.

    .payment | .customer_id, sum(.amount) |
      group_by(.customer_id) | having(sum(.amount) > 100)
*/

HAVING: 'having';
having: HAVING '(' expr ')';

/*
order_by
------

The 'order_by' construct implements the SQL "ORDER BY" clause.

    .actor | order_by(.first_name, .last_name)
    .actor | order_by(.first_name+)
    .actor | order_by(.actor.first_name-)

The optional plus/minus tokens specify ASC or DESC order.

Synonyms:

- 'sort_by' for jq interoperability.
  https://jqlang.github.io/jq/manual/v1.6/#sort,sort_by(path_expression)
- 'ob' for brevity.

We do not implement a 'sort' synonym for the jq 'sort' function, because SQL
results are inherently sorted. Although perhaps it should be implemented
as a no-op.
*/

ORDER_BY: 'order_by' | 'sort_by' | 'ob';
orderByTerm: selector ('+' | '-')?;
orderBy: ORDER_BY '(' orderByTerm (',' orderByTerm)* ')';

// selector specfies a table name, a column name, or table.column.
// - .first_name
// - ."first name"
// - .actor
// - ."actor"
// - .actor.first_name
selector: NAME (NAME)?;

// selector is a selector element.
// - .first_name
// - ."first name"
// - .first_name:given_name
// - ."first name":given_name
// - .actor.first_name
// - .actor.first_name:given_name
// - ."actor".first_name
selectorElement: (selector) (alias)?;

alias: ALIAS_RESERVED | ':' (ARG | ID | STRING);
// The grammar has problems dealing with "reserved" lexer tokens.
// Basically, there's a problem with using "column:KEYWORD".
// ALIAS_RESERVED is a hack to deal with those cases.
// The grammar could probably be refactored to not need this.
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

ARG: '$' ID;

arg : ARG;

// handleTable is a handle.table pair.
// - @my1.user
handleTable: HANDLE NAME;

// handle is a source handle.
// - @sakila
// - @work/acme/sakila
// - @home/csv/actor
handle: HANDLE;

// rowRange specifies a range of rows. It gets turned into
// a SQL "LIMIT x OFFSET y".
// - [] select all rows
// - [10] select row 10
// - [10:15] select rows 10 thru 15
// - [0:15] select rows 0 thru 15
// - [:15] same as above (0 thru 15) [10:] select all rows from 10 onwards
rowRange:
	'.[' (
		NN COLON NN // [10:15]
		| NN COLON // [10:]
		| COLON NN // [:15]
		| NN // [10]
	)? ']';


exprElement: expr (alias)?;

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

literal: NN | NUMBER | STRING | NULL;

unaryOperator: '-' | '+' | '~' | '!';

NULL: 'null';
ID: [a-zA-Z_][a-zA-Z0-9_]*;
WS: [ \t\r\n]+ -> skip;
LPAR: '(';
RPAR: ')';
LBRA: '[';
RBRA: ']';
COMMA: ',';
PIPE: '|';
COLON: ':';


// NN: Natural Number {0,1,2,3, ...}
NN: INTF;

NUMBER:
	NN
	| '-'? INTF '.' [0-9]+ EXP? // 1.35, 1.35E-9, 0.3, -4.5
	| '-'? INTF EXP // 1e10 -3e4
	| '-'? INTF ; // -3, 45

fragment INTF: '0' | [1-9] [0-9]*; // no leading zeros

fragment EXP:
	[Ee] [+\-]? INTF; // \- since "-" means "range" inside [...]

LT_EQ: '<=';
LT: '<';
GT_EQ: '>=';
GT: '>';
NEQ: '!=';
EQ: '==';


NAME: '.' (ARG | ID | STRING);

// SEL can be .THING or .THING.OTHERTHING.
// It can also be ."some name".OTHERTHING, etc.
//SEL: '.' (ID | STRING) ('.' (ID | STRING))*;

// HANDLE: @mydb1 or @postgres_db2 etc.

HANDLE: '@' ID ('/' ID)*;

STRING: '"' (ESC | ~["\\])* '"';
fragment ESC: '\\' (["\\/bfnrt] | UNICODE);
fragment UNICODE: 'u' HEX HEX HEX HEX;
fragment HEX: [0-9a-fA-F];

//NUMERIC_LITERAL
// : DIGIT+ ( '.' DIGIT* )? ( E [-+]? DIGIT+ )? | '.' DIGIT+ ( E [-+]? DIGIT+ )? ;

fragment DIGIT: [0-9];

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

LINECOMMENT: '#' .*? '\n' -> skip;

//// From https://github.com/antlr/grammars-v4/blob/master/sql/sqlite/SQLiteLexer.g4
//IDENTIFIER:
//    '"' (~'"' | '""')* '"'
//    | '`' (~'`' | '``')* '`'
//    | '[' ~']'* ']'
//    | [A-Z_] [A-Z_0-9]*
//; // TODO check: needs more chars in set


