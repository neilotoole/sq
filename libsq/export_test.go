package libsq

import (
	"context"

	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
)

// This file exports internal join-copy fan-in symbols for white-box testing
// from the external libsq_test package (which can import testh; the internal
// libsq test package cannot, as testh imports libsq).

// CopyBuffer is exported for testing.
type CopyBuffer = copyBuffer

// NewCopyBuffer is exported for testing.
func NewCopyBuffer(bufSize int) *CopyBuffer { return newCopyBuffer(bufSize) }

// FinishRead is exported for testing; it records the terminal read error.
func (b *copyBuffer) FinishRead(err error) { b.finish(err) }

// JoinCopyTask is exported for testing.
type JoinCopyTask = joinCopyTask

// NewJoinCopyTask is exported for testing.
func NewJoinCopyTask(fromGrip driver.Grip, fromTbl tablefq.T,
	toGrip driver.Grip, toTbl tablefq.T,
) *JoinCopyTask {
	return &joinCopyTask{fromGrip: fromGrip, fromTbl: fromTbl, toGrip: toGrip, toTbl: toTbl}
}

// WriteCopyTable is exported for testing.
func WriteCopyTable(ctx context.Context, task *JoinCopyTask, buf *CopyBuffer) error {
	return writeCopyTable(ctx, task, buf)
}

// ExecuteCopyTasksFanIn is exported for testing.
func ExecuteCopyTasksFanIn(ctx context.Context, tasks []*JoinCopyTask) error {
	return executeCopyTasksFanIn(ctx, tasks)
}
