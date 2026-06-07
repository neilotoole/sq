// Code generated from SQLiteParser.g4 by ANTLR 4.13.0. DO NOT EDIT.

package sqlite // SQLiteParser
import "github.com/antlr4-go/antlr/v4"

// A complete Visitor for a parse tree produced by SQLiteParser.
type SQLiteParserVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by SQLiteParser#parse.
	VisitParse(ctx *ParseContext) interface{}

	// Visit a parse tree produced by SQLiteParser#sql_stmt_list.
	VisitSql_stmt_list(ctx *Sql_stmt_listContext) interface{}

	// Visit a parse tree produced by SQLiteParser#sql_stmt.
	VisitSql_stmt(ctx *Sql_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#alter_table_stmt.
	VisitAlter_table_stmt(ctx *Alter_table_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#analyze_stmt.
	VisitAnalyze_stmt(ctx *Analyze_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#attach_stmt.
	VisitAttach_stmt(ctx *Attach_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#begin_stmt.
	VisitBegin_stmt(ctx *Begin_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#commit_stmt.
	VisitCommit_stmt(ctx *Commit_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#rollback_stmt.
	VisitRollback_stmt(ctx *Rollback_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#savepoint_stmt.
	VisitSavepoint_stmt(ctx *Savepoint_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#release_stmt.
	VisitRelease_stmt(ctx *Release_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#create_index_stmt.
	VisitCreate_index_stmt(ctx *Create_index_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#indexed_column.
	VisitIndexed_column(ctx *Indexed_columnContext) interface{}

	// Visit a parse tree produced by SQLiteParser#create_table_stmt.
	VisitCreate_table_stmt(ctx *Create_table_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#column_def.
	VisitColumn_def(ctx *Column_defContext) interface{}

	// Visit a parse tree produced by SQLiteParser#type_name.
	VisitType_name(ctx *Type_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#column_constraint.
	VisitColumn_constraint(ctx *Column_constraintContext) interface{}

	// Visit a parse tree produced by SQLiteParser#signed_number.
	VisitSigned_number(ctx *Signed_numberContext) interface{}

	// Visit a parse tree produced by SQLiteParser#table_constraint.
	VisitTable_constraint(ctx *Table_constraintContext) interface{}

	// Visit a parse tree produced by SQLiteParser#foreign_key_clause.
	VisitForeign_key_clause(ctx *Foreign_key_clauseContext) interface{}

	// Visit a parse tree produced by SQLiteParser#conflict_clause.
	VisitConflict_clause(ctx *Conflict_clauseContext) interface{}

	// Visit a parse tree produced by SQLiteParser#create_trigger_stmt.
	VisitCreate_trigger_stmt(ctx *Create_trigger_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#create_view_stmt.
	VisitCreate_view_stmt(ctx *Create_view_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#create_virtual_table_stmt.
	VisitCreate_virtual_table_stmt(ctx *Create_virtual_table_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#with_clause.
	VisitWith_clause(ctx *With_clauseContext) interface{}

	// Visit a parse tree produced by SQLiteParser#cte_table_name.
	VisitCte_table_name(ctx *Cte_table_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#recursive_cte.
	VisitRecursive_cte(ctx *Recursive_cteContext) interface{}

	// Visit a parse tree produced by SQLiteParser#common_table_expression.
	VisitCommon_table_expression(ctx *Common_table_expressionContext) interface{}

	// Visit a parse tree produced by SQLiteParser#delete_stmt.
	VisitDelete_stmt(ctx *Delete_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#delete_stmt_limited.
	VisitDelete_stmt_limited(ctx *Delete_stmt_limitedContext) interface{}

	// Visit a parse tree produced by SQLiteParser#detach_stmt.
	VisitDetach_stmt(ctx *Detach_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#drop_stmt.
	VisitDrop_stmt(ctx *Drop_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#expr.
	VisitExpr(ctx *ExprContext) interface{}

	// Visit a parse tree produced by SQLiteParser#raise_function.
	VisitRaise_function(ctx *Raise_functionContext) interface{}

	// Visit a parse tree produced by SQLiteParser#literal_value.
	VisitLiteral_value(ctx *Literal_valueContext) interface{}

	// Visit a parse tree produced by SQLiteParser#value_row.
	VisitValue_row(ctx *Value_rowContext) interface{}

	// Visit a parse tree produced by SQLiteParser#values_clause.
	VisitValues_clause(ctx *Values_clauseContext) interface{}

	// Visit a parse tree produced by SQLiteParser#insert_stmt.
	VisitInsert_stmt(ctx *Insert_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#returning_clause.
	VisitReturning_clause(ctx *Returning_clauseContext) interface{}

	// Visit a parse tree produced by SQLiteParser#upsert_clause.
	VisitUpsert_clause(ctx *Upsert_clauseContext) interface{}

	// Visit a parse tree produced by SQLiteParser#pragma_stmt.
	VisitPragma_stmt(ctx *Pragma_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#pragma_value.
	VisitPragma_value(ctx *Pragma_valueContext) interface{}

	// Visit a parse tree produced by SQLiteParser#reindex_stmt.
	VisitReindex_stmt(ctx *Reindex_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#select_stmt.
	VisitSelect_stmt(ctx *Select_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#join_clause.
	VisitJoin_clause(ctx *Join_clauseContext) interface{}

	// Visit a parse tree produced by SQLiteParser#select_core.
	VisitSelect_core(ctx *Select_coreContext) interface{}

	// Visit a parse tree produced by SQLiteParser#factored_select_stmt.
	VisitFactored_select_stmt(ctx *Factored_select_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#simple_select_stmt.
	VisitSimple_select_stmt(ctx *Simple_select_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#compound_select_stmt.
	VisitCompound_select_stmt(ctx *Compound_select_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#table_or_subquery.
	VisitTable_or_subquery(ctx *Table_or_subqueryContext) interface{}

	// Visit a parse tree produced by SQLiteParser#result_column.
	VisitResult_column(ctx *Result_columnContext) interface{}

	// Visit a parse tree produced by SQLiteParser#join_operator.
	VisitJoin_operator(ctx *Join_operatorContext) interface{}

	// Visit a parse tree produced by SQLiteParser#join_constraint.
	VisitJoin_constraint(ctx *Join_constraintContext) interface{}

	// Visit a parse tree produced by SQLiteParser#compound_operator.
	VisitCompound_operator(ctx *Compound_operatorContext) interface{}

	// Visit a parse tree produced by SQLiteParser#update_stmt.
	VisitUpdate_stmt(ctx *Update_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#column_name_list.
	VisitColumn_name_list(ctx *Column_name_listContext) interface{}

	// Visit a parse tree produced by SQLiteParser#update_stmt_limited.
	VisitUpdate_stmt_limited(ctx *Update_stmt_limitedContext) interface{}

	// Visit a parse tree produced by SQLiteParser#qualified_table_name.
	VisitQualified_table_name(ctx *Qualified_table_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#vacuum_stmt.
	VisitVacuum_stmt(ctx *Vacuum_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#filter_clause.
	VisitFilter_clause(ctx *Filter_clauseContext) interface{}

	// Visit a parse tree produced by SQLiteParser#window_defn.
	VisitWindow_defn(ctx *Window_defnContext) interface{}

	// Visit a parse tree produced by SQLiteParser#over_clause.
	VisitOver_clause(ctx *Over_clauseContext) interface{}

	// Visit a parse tree produced by SQLiteParser#frame_spec.
	VisitFrame_spec(ctx *Frame_specContext) interface{}

	// Visit a parse tree produced by SQLiteParser#frame_clause.
	VisitFrame_clause(ctx *Frame_clauseContext) interface{}

	// Visit a parse tree produced by SQLiteParser#simple_function_invocation.
	VisitSimple_function_invocation(ctx *Simple_function_invocationContext) interface{}

	// Visit a parse tree produced by SQLiteParser#aggregate_function_invocation.
	VisitAggregate_function_invocation(ctx *Aggregate_function_invocationContext) interface{}

	// Visit a parse tree produced by SQLiteParser#window_function_invocation.
	VisitWindow_function_invocation(ctx *Window_function_invocationContext) interface{}

	// Visit a parse tree produced by SQLiteParser#common_table_stmt.
	VisitCommon_table_stmt(ctx *Common_table_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#order_by_stmt.
	VisitOrder_by_stmt(ctx *Order_by_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#limit_stmt.
	VisitLimit_stmt(ctx *Limit_stmtContext) interface{}

	// Visit a parse tree produced by SQLiteParser#ordering_term.
	VisitOrdering_term(ctx *Ordering_termContext) interface{}

	// Visit a parse tree produced by SQLiteParser#asc_desc.
	VisitAsc_desc(ctx *Asc_descContext) interface{}

	// Visit a parse tree produced by SQLiteParser#frame_left.
	VisitFrame_left(ctx *Frame_leftContext) interface{}

	// Visit a parse tree produced by SQLiteParser#frame_right.
	VisitFrame_right(ctx *Frame_rightContext) interface{}

	// Visit a parse tree produced by SQLiteParser#frame_single.
	VisitFrame_single(ctx *Frame_singleContext) interface{}

	// Visit a parse tree produced by SQLiteParser#window_function.
	VisitWindow_function(ctx *Window_functionContext) interface{}

	// Visit a parse tree produced by SQLiteParser#offset.
	VisitOffset(ctx *OffsetContext) interface{}

	// Visit a parse tree produced by SQLiteParser#default_value.
	VisitDefault_value(ctx *Default_valueContext) interface{}

	// Visit a parse tree produced by SQLiteParser#partition_by.
	VisitPartition_by(ctx *Partition_byContext) interface{}

	// Visit a parse tree produced by SQLiteParser#order_by_expr.
	VisitOrder_by_expr(ctx *Order_by_exprContext) interface{}

	// Visit a parse tree produced by SQLiteParser#order_by_expr_asc_desc.
	VisitOrder_by_expr_asc_desc(ctx *Order_by_expr_asc_descContext) interface{}

	// Visit a parse tree produced by SQLiteParser#expr_asc_desc.
	VisitExpr_asc_desc(ctx *Expr_asc_descContext) interface{}

	// Visit a parse tree produced by SQLiteParser#initial_select.
	VisitInitial_select(ctx *Initial_selectContext) interface{}

	// Visit a parse tree produced by SQLiteParser#recursive_select.
	VisitRecursive_select(ctx *Recursive_selectContext) interface{}

	// Visit a parse tree produced by SQLiteParser#unary_operator.
	VisitUnary_operator(ctx *Unary_operatorContext) interface{}

	// Visit a parse tree produced by SQLiteParser#error_message.
	VisitError_message(ctx *Error_messageContext) interface{}

	// Visit a parse tree produced by SQLiteParser#module_argument.
	VisitModule_argument(ctx *Module_argumentContext) interface{}

	// Visit a parse tree produced by SQLiteParser#column_alias.
	VisitColumn_alias(ctx *Column_aliasContext) interface{}

	// Visit a parse tree produced by SQLiteParser#keyword.
	VisitKeyword(ctx *KeywordContext) interface{}

	// Visit a parse tree produced by SQLiteParser#name.
	VisitName(ctx *NameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#function_name.
	VisitFunction_name(ctx *Function_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#schema_name.
	VisitSchema_name(ctx *Schema_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#table_name.
	VisitTable_name(ctx *Table_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#table_or_index_name.
	VisitTable_or_index_name(ctx *Table_or_index_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#column_name.
	VisitColumn_name(ctx *Column_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#collation_name.
	VisitCollation_name(ctx *Collation_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#foreign_table.
	VisitForeign_table(ctx *Foreign_tableContext) interface{}

	// Visit a parse tree produced by SQLiteParser#index_name.
	VisitIndex_name(ctx *Index_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#trigger_name.
	VisitTrigger_name(ctx *Trigger_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#view_name.
	VisitView_name(ctx *View_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#module_name.
	VisitModule_name(ctx *Module_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#pragma_name.
	VisitPragma_name(ctx *Pragma_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#savepoint_name.
	VisitSavepoint_name(ctx *Savepoint_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#table_alias.
	VisitTable_alias(ctx *Table_aliasContext) interface{}

	// Visit a parse tree produced by SQLiteParser#transaction_name.
	VisitTransaction_name(ctx *Transaction_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#window_name.
	VisitWindow_name(ctx *Window_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#alias.
	VisitAlias(ctx *AliasContext) interface{}

	// Visit a parse tree produced by SQLiteParser#filename.
	VisitFilename(ctx *FilenameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#base_window_name.
	VisitBase_window_name(ctx *Base_window_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#simple_func.
	VisitSimple_func(ctx *Simple_funcContext) interface{}

	// Visit a parse tree produced by SQLiteParser#aggregate_func.
	VisitAggregate_func(ctx *Aggregate_funcContext) interface{}

	// Visit a parse tree produced by SQLiteParser#table_function_name.
	VisitTable_function_name(ctx *Table_function_nameContext) interface{}

	// Visit a parse tree produced by SQLiteParser#any_name.
	VisitAny_name(ctx *Any_nameContext) interface{}
}
