// Code generated from SQLiteParser.g4 by ANTLR 4.13.0. DO NOT EDIT.

package sqlite // SQLiteParser
import "github.com/antlr4-go/antlr/v4"

type BaseSQLiteParserVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseSQLiteParserVisitor) VisitParse(ctx *ParseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitSql_stmt_list(ctx *Sql_stmt_listContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitSql_stmt(ctx *Sql_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitAlter_table_stmt(ctx *Alter_table_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitAnalyze_stmt(ctx *Analyze_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitAttach_stmt(ctx *Attach_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitBegin_stmt(ctx *Begin_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitCommit_stmt(ctx *Commit_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitRollback_stmt(ctx *Rollback_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitSavepoint_stmt(ctx *Savepoint_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitRelease_stmt(ctx *Release_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitCreate_index_stmt(ctx *Create_index_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitIndexed_column(ctx *Indexed_columnContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitCreate_table_stmt(ctx *Create_table_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitColumn_def(ctx *Column_defContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitType_name(ctx *Type_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitColumn_constraint(ctx *Column_constraintContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitSigned_number(ctx *Signed_numberContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitTable_constraint(ctx *Table_constraintContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitForeign_key_clause(ctx *Foreign_key_clauseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitConflict_clause(ctx *Conflict_clauseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitCreate_trigger_stmt(ctx *Create_trigger_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitCreate_view_stmt(ctx *Create_view_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitCreate_virtual_table_stmt(ctx *Create_virtual_table_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitWith_clause(ctx *With_clauseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitCte_table_name(ctx *Cte_table_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitRecursive_cte(ctx *Recursive_cteContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitCommon_table_expression(ctx *Common_table_expressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitDelete_stmt(ctx *Delete_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitDelete_stmt_limited(ctx *Delete_stmt_limitedContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitDetach_stmt(ctx *Detach_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitDrop_stmt(ctx *Drop_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitExpr(ctx *ExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitRaise_function(ctx *Raise_functionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitLiteral_value(ctx *Literal_valueContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitValue_row(ctx *Value_rowContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitValues_clause(ctx *Values_clauseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitInsert_stmt(ctx *Insert_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitReturning_clause(ctx *Returning_clauseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitUpsert_clause(ctx *Upsert_clauseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitPragma_stmt(ctx *Pragma_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitPragma_value(ctx *Pragma_valueContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitReindex_stmt(ctx *Reindex_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitSelect_stmt(ctx *Select_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitJoin_clause(ctx *Join_clauseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitSelect_core(ctx *Select_coreContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitFactored_select_stmt(ctx *Factored_select_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitSimple_select_stmt(ctx *Simple_select_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitCompound_select_stmt(ctx *Compound_select_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitTable_or_subquery(ctx *Table_or_subqueryContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitResult_column(ctx *Result_columnContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitJoin_operator(ctx *Join_operatorContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitJoin_constraint(ctx *Join_constraintContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitCompound_operator(ctx *Compound_operatorContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitUpdate_stmt(ctx *Update_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitColumn_name_list(ctx *Column_name_listContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitUpdate_stmt_limited(ctx *Update_stmt_limitedContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitQualified_table_name(ctx *Qualified_table_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitVacuum_stmt(ctx *Vacuum_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitFilter_clause(ctx *Filter_clauseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitWindow_defn(ctx *Window_defnContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitOver_clause(ctx *Over_clauseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitFrame_spec(ctx *Frame_specContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitFrame_clause(ctx *Frame_clauseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitSimple_function_invocation(ctx *Simple_function_invocationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitAggregate_function_invocation(ctx *Aggregate_function_invocationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitWindow_function_invocation(ctx *Window_function_invocationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitCommon_table_stmt(ctx *Common_table_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitOrder_by_stmt(ctx *Order_by_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitLimit_stmt(ctx *Limit_stmtContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitOrdering_term(ctx *Ordering_termContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitAsc_desc(ctx *Asc_descContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitFrame_left(ctx *Frame_leftContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitFrame_right(ctx *Frame_rightContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitFrame_single(ctx *Frame_singleContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitWindow_function(ctx *Window_functionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitOffset(ctx *OffsetContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitDefault_value(ctx *Default_valueContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitPartition_by(ctx *Partition_byContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitOrder_by_expr(ctx *Order_by_exprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitOrder_by_expr_asc_desc(ctx *Order_by_expr_asc_descContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitExpr_asc_desc(ctx *Expr_asc_descContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitInitial_select(ctx *Initial_selectContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitRecursive_select(ctx *Recursive_selectContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitUnary_operator(ctx *Unary_operatorContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitError_message(ctx *Error_messageContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitModule_argument(ctx *Module_argumentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitColumn_alias(ctx *Column_aliasContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitKeyword(ctx *KeywordContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitName(ctx *NameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitFunction_name(ctx *Function_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitSchema_name(ctx *Schema_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitTable_name(ctx *Table_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitTable_or_index_name(ctx *Table_or_index_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitColumn_name(ctx *Column_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitCollation_name(ctx *Collation_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitForeign_table(ctx *Foreign_tableContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitIndex_name(ctx *Index_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitTrigger_name(ctx *Trigger_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitView_name(ctx *View_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitModule_name(ctx *Module_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitPragma_name(ctx *Pragma_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitSavepoint_name(ctx *Savepoint_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitTable_alias(ctx *Table_aliasContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitTransaction_name(ctx *Transaction_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitWindow_name(ctx *Window_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitAlias(ctx *AliasContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitFilename(ctx *FilenameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitBase_window_name(ctx *Base_window_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitSimple_func(ctx *Simple_funcContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitAggregate_func(ctx *Aggregate_funcContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitTable_function_name(ctx *Table_function_nameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQLiteParserVisitor) VisitAny_name(ctx *Any_nameContext) interface{} {
	return v.VisitChildren(ctx)
}
