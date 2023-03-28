// This is the grammar for SLQ, the query language used by sq (https://sq.io).
// The grammar is not yet finalized; it is subject to change in any new sq release.
grammar SLQ;

// alias, for columns, implements "col AS alias".
// For example: ".first_name:given_name" : "given_name" is the alias.


stmtList: ';'* query ( ';'+ query)* ';'*;

query: segment ('|' segment)*;

segment: (element) (',' element)*;

element
    : handleTable
	| handle
	| selectorElement
	| join
	| groupBy
	| orderBy
	| rowRange
	| uniqueFunc
	| countFunc
	| funcElement
	| expr;

// cmpr is a comparison operator.
cmpr: LT_EQ | LT | GT_EQ | GT | EQ | NEQ;



funcElement: func (alias)?;
func: funcName '(' ( expr ( ',' expr)* | '*')? ')';
funcName: ID;

join: ('join') '(' joinConstraint ')';

joinConstraint
    : selector cmpr selector // .user.uid == .address.userid
	| selector ; // .uid

/*
uniqueFunc
----------

uniqueFunc implements SQL's DISTINCT mechanism.

    .actor | .first_name | unique
    .actor | unique

The func takes zero args.
*/
uniqueFunc: 'unique';

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

 TODO: how to handle COUNT DISTINCT?
 */
countFunc:
//    : COUNT (LPAR (selector)? RPAR)?;
//    : 'count' (LPAR (selector)? RPAR)? (ALIAS)?
//    : 'count:count' // Deal with some pathological cases.
//    | 'count():count'
//    | 'count' (LPAR (selector)? RPAR)? (alias)?

//    | 'count' (LPAR (selector)? RPAR)? (':count')?
//    | 'count' (LPAR (selector)? RPAR)? (ALIAS_RESERVED)?
    | 'count' (LPAR (selector)? RPAR)? (alias)?
    ;



//COUNT: 'count';


//ALIAS: ':' [a-zA-Z_][a-zA-Z0-9_]*;

/*
group_by
-------

The 'group_by' construct implments the SQL "GROUP BY" clause.

    .payment | .customer_id, sum(.amount) | group_by(.customer_id)

Syonyms:
- 'group_by' for jq interoperability.
  https://stedolan.github.io/jq/manual/v1.6/#group_by(path_expression)
- 'group': for legacy sq compabibility. Should this be deprecated and removed?
*/

GROUP_BY: 'group_by';
groupByTerm: selector | func;
groupBy: GROUP_BY '(' groupByTerm (',' groupByTerm)* ')';

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
  https://stedolan.github.io/jq/manual/v1.6/#sort,sort_by(path_expression)

We do not implement a 'sort' synonym for the jq 'sort' function, because SQL
results are inherently sorted. Although perhaps it should be implemented
as a no-op.
*/

ORDER_ASC: '+';
ORDER_DESC: '-';
ORDER_BY: 'order_by' | 'sort_by';
orderByTerm: selector (ORDER_ASC | ORDER_DESC)?;
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
selectorElement: selector (alias)?;

alias: ALIAS_RESERVED | ':' ID;
// The grammar has problems dealing with "reserved" lexer tokens.
// Basically, there's a problem with using "column:KEYWORD".
// ALIAS_RESERVED is a hack to deal with those cases.
// The grammar could probably be refactored to not need this.
ALIAS_RESERVED
    // TODO: Update ALIAS_RESERVED with all "keywords"
    : ':count'
    | ':avg'
    | ':group_by'
    | ':max'
    | ':min'
    | ':order_by'
    | ':unique'
    ;

// handleTable is a handle.table pair.
// - @my1.user
handleTable: HANDLE NAME;

// handle is a source handle.
// - @sakila
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

//fnName:
//	'sum'
//	| 'SUM'
//	| 'avg'
//	| 'AVG'
//	| 'count'
//	| 'COUNT'
//	| 'where'
//	| 'WHERE';



expr:
	selector
	| literal
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

ID: [a-zA-Z_][a-zA-Z0-9_]*;
WS: [ \t\r\n]+ -> skip;
LPAR: '(';
RPAR: ')';
LBRA: '[';
RBRA: ']';
COMMA: ',';
PIPE: '|';
COLON: ':';
NULL: 'null' | 'NULL';

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


NAME: '.' (ID | STRING);

// SEL can be .THING or .THING.OTHERTHING.
// It can also be ."some name".OTHERTHING, etc.
//SEL: '.' (ID | STRING) ('.' (ID | STRING))*;

// HANDLE: @mydb1 or @postgres_db2 etc.
HANDLE: '@' ID;

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


