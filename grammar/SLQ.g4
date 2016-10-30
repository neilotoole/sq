grammar SLQ;

// "@mysql_db1 | .user, .address | join(.user.uid == .address.uid) | .[0:3] | .uid, .username, .country"


query: segment (PIPE segment)* ;

segment: (element)  (COMMA element)* ;

element: dsTblElement | dsElement | selElement | join | rowRange;



cmpr: LT_EQ | LT | GT_EQ | GT | EQ | NEQ ;

//fn: fnJoin ;

args: ( arg (COMMA arg)* )? ;

arg: SEL | ID ;


join
 : ('join'|'JOIN'|'j')
 LPAR joinConstraint RPAR
 ;
//fnJoinCond: SEL cmpr SEL ;
//fnJoinExpr: fnJoinCond | SEL;
joinConstraint
 : SEL cmpr SEL // .user.uid == .address.userid
 | SEL // .uid
 ;

selElement: SEL;

dsTblElement: DATASOURCE SEL; // datasource table element, e.g. @my1.user

dsElement: DATASOURCE; // datasource element, e.g. @my1

// []
// [1] select row[1]
// [10:15] select rows 10 thru 15
// [0:15] select rows 0 thru 15
// [:15] same as above (0 thru 15)
// [10:] select all rows from 10 onwards
rowRange: '.[' (( INT COLON INT) | (INT COLON) | (COLON INT) | INT | ) ']'; // [1] [10:15] [0:15] [:15] [10:]

ID: [a-zA-Z_][a-zA-Z0-9_]* ;
WS : [ \t\r\n]+ -> skip ;
LPAR : '(' ;
RPAR: ')';
LBRA: '[' ;
RBRA: ']';
COMMA: ',';
PIPE: '|' ;
COLON: ':';
NULL: 'null' | 'NULL';
STRING :  '"' (ESC | ~["\\])* '"' ;

fragment ESC :   '\\' (["\\/bfnrt] | UNICODE) ;
fragment UNICODE : 'u' HEX HEX HEX HEX ;
fragment HEX : [0-9a-fA-F] ;

INT:  [0-9] [0-9]*;

NUMBER
    :   '-'? INTF '.' [0-9]+ EXP? // 1.35, 1.35E-9, 0.3, -4.5
    |   '-'? INTF EXP             // 1e10 -3e4
    |   '-'? INTF                 // -3, 45
    ;
fragment INTF :   '0' | [1-9] [0-9]* ; // no leading zeros
fragment EXP :   [Ee] [+\-]? INTF ; // \- since - means "range" inside [...]


LT_EQ : '<=';
LT : '<';
GT_EQ : '>=';
GT : '>';
NEQ : '!=';
EQ : '==';

SEL: '.' ID ('.' ID)*; // SEL can be .THING or .THING.OTHERTHING etc.
DATASOURCE: '@' ID; // DS is Datasource
DOT: '.' ;
VAL: STRING | NUMBER | NULL | SEL;

LINECOMMENT: '//' .*? '\n' -> skip ;