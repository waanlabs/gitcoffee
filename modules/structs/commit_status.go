// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// CommitStatusState holds the state of a CommitStatus
// It can be "pending", "success", "error" and "failure"
type CommitStatusState string

const (
	// CommitStatusPending is for when the CommitStatus is Pending
	CommitStatusPending CommitStatusState = "pending"
	// CommitStatusSuccess is for when the CommitStatus is Success
	CommitStatusSuccess CommitStatusState = "success"
	// CommitStatusError is for when the CommitStatus is Error
	CommitStatusError CommitStatusState = "error"
	// CommitStatusFailure is for when the CommitStatus is Failure
	CommitStatusFailure CommitStatusState = "failure"
)

var commitStatusPriorities = map[CommitStatusState]int{
	CommitStatusError:   0,
	CommitStatusFailure: 1,
	CommitStatusPending: 2,
	CommitStatusSuccess: 3,
}

// NoBetterThan returns true if this State is no better than the given State
// This function only handles the states defined in CommitStatusPriorities
func (css CommitStatusState) NoBetterThan(css2 CommitStatusState) bool {
	// NoBetterThan only handles the 4 states above
	if _, exist := commitStatusPriorities[css]; !exist {
		return false
	}

	if _, exist := commitStatusPriorities[css2]; !exist {
		return false
	}

	return commitStatusPriorities[css] <= commitStatusPriorities[css2]
}

// IsPending represents if commit status state is pending
func (css CommitStatusState) IsPending() bool {
	return css == CommitStatusPending
}

// IsSuccess represents if commit status state is success
func (css CommitStatusState) IsSuccess() bool {
	return css == CommitStatusSuccess
}

// IsError represents if commit status state is error
func (css CommitStatusState) IsError() bool {
	return css == CommitStatusError
}

// IsFailure represents if commit status state is failure
func (css CommitStatusState) IsFailure() bool {
	return css == CommitStatusFailure
}
