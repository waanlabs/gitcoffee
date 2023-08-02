// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
)

// ChangeStatus changes issue status to open or closed.
func ChangeStatus(issue *issues_model.Issue, doer *user_model.User, commitID string, closed bool) error {
	return changeStatusCtx(db.DefaultContext, issue, doer, commitID, closed)
}

// changeStatusCtx changes issue status to open or closed.
// TODO: if context is not db.DefaultContext we get a deadlock!!!
func changeStatusCtx(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, commitID string, closed bool) error {
	comment, err := issues_model.ChangeIssueStatus(ctx, issue, doer, closed)
	if err != nil {
		if issues_model.IsErrDependenciesLeft(err) && closed {
			if err := issues_model.FinishIssueStopwatchIfPossible(ctx, doer, issue); err != nil {
				log.Error("Unable to stop stopwatch for issue[%d]#%d: %v", issue.ID, issue.Index, err)
			}
		}
		return err
	}

	if closed {
		if err := issues_model.FinishIssueStopwatchIfPossible(ctx, doer, issue); err != nil {
			return err
		}
	}

	notification.NotifyIssueChangeStatus(ctx, doer, commitID, issue, comment, closed)

	return nil
}
