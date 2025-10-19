package entity

import "github.com/HMasataka/errs"

// Assignment related errors
var (
	ErrAssignmentNotFound     *errs.Error = errs.New("assignment not found")
	ErrAssignmentGetFailed    *errs.Error = errs.New("failed to get assignment")
	ErrAssignmentDecodeFailed *errs.Error = errs.New("failed to decode assignment")
	ErrAssignmentEncodeFailed *errs.Error = errs.New("failed to encode assignment")
	ErrAssignmentSetFailed    *errs.Error = errs.New("failed to set assignment data")
	ErrAssignmentWatchFailed  *errs.Error = errs.New("failed to watch assignments")
)

// Ticket related errors
var (
	ErrTicketGetFailed        *errs.Error = errs.New("failed to get ticket")
	ErrTicketCreateFailed     *errs.Error = errs.New("failed to create ticket")
	ErrTicketDeleteFailed     *errs.Error = errs.New("failed to delete ticket")
	ErrTicketMarshalFailed    *errs.Error = errs.New("failed to marshal ticket")
	ErrTicketUnmarshalFailed  *errs.Error = errs.New("failed to unmarshal ticket")
	ErrTicketDeindexFailed    *errs.Error = errs.New("failed to deindex tickets")
	ErrTicketExpirationFailed *errs.Error = errs.New("failed to set ticket expiration")
)

// Lock related errors
var (
	ErrLockAcquisitionFailed *errs.Error = errs.New("failed to acquire lock")
)

// Index related errors
var (
	ErrIndexGetFailed    *errs.Error = errs.New("failed to get index")
	ErrIndexDecodeFailed *errs.Error = errs.New("failed to decode index")
	ErrIndexDeleteFailed *errs.Error = errs.New("failed to delete index")
)

// Match related errors
var (
	ErrMatchExecutionFailed  *errs.Error = errs.New("failed to execute match functions")
	ErrMatchEvaluationFailed *errs.Error = errs.New("failed to evaluate matches")
	ErrMatchAssignFailed     *errs.Error = errs.New("failed to assign matches")
)

// Pending ticket related errors
var (
	ErrPendingTicketGetFailed     *errs.Error = errs.New("failed to get pending tickets")
	ErrPendingTicketSetFailed     *errs.Error = errs.New("failed to set pending tickets")
	ErrPendingTicketReleaseFailed *errs.Error = errs.New("failed to release tickets")
)

// Redis operation errors
var (
	ErrRedisOperationFailed *errs.Error = errs.New("redis operation failed")
)
