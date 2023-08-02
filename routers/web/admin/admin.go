// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/updatechecker"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/cron"
	"code.gitea.io/gitea/services/forms"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplDashboard   base.TplName = "admin/dashboard"
	tplCron        base.TplName = "admin/cron"
	tplQueue       base.TplName = "admin/queue"
	tplStacktrace  base.TplName = "admin/stacktrace"
	tplQueueManage base.TplName = "admin/queue_manage"
	tplStats       base.TplName = "admin/stats"
)

var sysStatus struct {
	StartTime    string
	NumGoroutine int

	// General statistics.
	MemAllocated string // bytes allocated and still in use
	MemTotal     string // bytes allocated (even if freed)
	MemSys       string // bytes obtained from system (sum of XxxSys below)
	Lookups      uint64 // number of pointer lookups
	MemMallocs   uint64 // number of mallocs
	MemFrees     uint64 // number of frees

	// Main allocation heap statistics.
	HeapAlloc    string // bytes allocated and still in use
	HeapSys      string // bytes obtained from system
	HeapIdle     string // bytes in idle spans
	HeapInuse    string // bytes in non-idle span
	HeapReleased string // bytes released to the OS
	HeapObjects  uint64 // total number of allocated objects

	// Low-level fixed-size structure allocator statistics.
	//	Inuse is bytes used now.
	//	Sys is bytes obtained from system.
	StackInuse  string // bootstrap stacks
	StackSys    string
	MSpanInuse  string // mspan structures
	MSpanSys    string
	MCacheInuse string // mcache structures
	MCacheSys   string
	BuckHashSys string // profiling bucket hash table
	GCSys       string // GC metadata
	OtherSys    string // other system allocations

	// Garbage collector statistics.
	NextGC       string // next run in HeapAlloc time (bytes)
	LastGC       string // last run in absolute time (ns)
	PauseTotalNs string
	PauseNs      string // circular buffer of recent GC pause times, most recent at [(NumGC+255)%256]
	NumGC        uint32
}

func updateSystemStatus() {
	sysStatus.StartTime = setting.AppStartTime.Format(time.RFC3339)

	m := new(runtime.MemStats)
	runtime.ReadMemStats(m)
	sysStatus.NumGoroutine = runtime.NumGoroutine()

	sysStatus.MemAllocated = base.FileSize(int64(m.Alloc))
	sysStatus.MemTotal = base.FileSize(int64(m.TotalAlloc))
	sysStatus.MemSys = base.FileSize(int64(m.Sys))
	sysStatus.Lookups = m.Lookups
	sysStatus.MemMallocs = m.Mallocs
	sysStatus.MemFrees = m.Frees

	sysStatus.HeapAlloc = base.FileSize(int64(m.HeapAlloc))
	sysStatus.HeapSys = base.FileSize(int64(m.HeapSys))
	sysStatus.HeapIdle = base.FileSize(int64(m.HeapIdle))
	sysStatus.HeapInuse = base.FileSize(int64(m.HeapInuse))
	sysStatus.HeapReleased = base.FileSize(int64(m.HeapReleased))
	sysStatus.HeapObjects = m.HeapObjects

	sysStatus.StackInuse = base.FileSize(int64(m.StackInuse))
	sysStatus.StackSys = base.FileSize(int64(m.StackSys))
	sysStatus.MSpanInuse = base.FileSize(int64(m.MSpanInuse))
	sysStatus.MSpanSys = base.FileSize(int64(m.MSpanSys))
	sysStatus.MCacheInuse = base.FileSize(int64(m.MCacheInuse))
	sysStatus.MCacheSys = base.FileSize(int64(m.MCacheSys))
	sysStatus.BuckHashSys = base.FileSize(int64(m.BuckHashSys))
	sysStatus.GCSys = base.FileSize(int64(m.GCSys))
	sysStatus.OtherSys = base.FileSize(int64(m.OtherSys))

	sysStatus.NextGC = base.FileSize(int64(m.NextGC))
	sysStatus.LastGC = fmt.Sprintf("%.1fs", float64(time.Now().UnixNano()-int64(m.LastGC))/1000/1000/1000)
	sysStatus.PauseTotalNs = fmt.Sprintf("%.1fs", float64(m.PauseTotalNs)/1000/1000/1000)
	sysStatus.PauseNs = fmt.Sprintf("%.3fs", float64(m.PauseNs[(m.NumGC+255)%256])/1000/1000/1000)
	sysStatus.NumGC = m.NumGC
}

func prepareDeprecatedWarningsAlert(ctx *context.Context) {
	if len(setting.DeprecatedWarnings) > 0 {
		content := setting.DeprecatedWarnings[0]
		if len(setting.DeprecatedWarnings) > 1 {
			content += fmt.Sprintf(" (and %d more)", len(setting.DeprecatedWarnings)-1)
		}
		ctx.Flash.Error(content, true)
	}
}

// Dashboard show admin panel dashboard
func Dashboard(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.dashboard")
	ctx.Data["PageIsAdminDashboard"] = true
	ctx.Data["NeedUpdate"] = updatechecker.GetNeedUpdate()
	ctx.Data["RemoteVersion"] = updatechecker.GetRemoteVersion()
	// FIXME: update periodically
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus
	ctx.Data["SSH"] = setting.SSH
	prepareDeprecatedWarningsAlert(ctx)
	ctx.HTML(http.StatusOK, tplDashboard)
}

// DashboardPost run an admin operation
func DashboardPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminDashboardForm)
	ctx.Data["Title"] = ctx.Tr("admin.dashboard")
	ctx.Data["PageIsAdminDashboard"] = true
	updateSystemStatus()
	ctx.Data["SysStatus"] = sysStatus

	// Run operation.
	if form.Op != "" {
		switch form.Op {
		case "sync_repo_branches":
			go func() {
				if err := repo_service.AddAllRepoBranchesToSyncQueue(graceful.GetManager().ShutdownContext(), ctx.Doer.ID); err != nil {
					log.Error("AddAllRepoBranchesToSyncQueue: %v: %v", ctx.Doer.ID, err)
				}
			}()
			ctx.Flash.Success(ctx.Tr("admin.dashboard.sync_branch.started"))
		default:
			task := cron.GetTask(form.Op)
			if task != nil {
				go task.RunWithUser(ctx.Doer, nil)
				ctx.Flash.Success(ctx.Tr("admin.dashboard.task.started", ctx.Tr("admin.dashboard."+form.Op)))
			} else {
				ctx.Flash.Error(ctx.Tr("admin.dashboard.task.unknown", form.Op))
			}
		}
	}
	if form.From == "monitor" {
		ctx.Redirect(setting.AppSubURL + "/admin/monitor/cron")
	} else {
		ctx.Redirect(setting.AppSubURL + "/admin")
	}
}

func CronTasks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor.cron")
	ctx.Data["PageIsAdminMonitorCron"] = true
	ctx.Data["Entries"] = cron.ListTasks()
	ctx.HTML(http.StatusOK, tplCron)
}

func MonitorStats(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.monitor.stats")
	ctx.Data["PageIsAdminMonitorStats"] = true
	bs, err := json.Marshal(activities_model.GetStatistic().Counter)
	if err != nil {
		ctx.ServerError("MonitorStats", err)
		return
	}
	statsCounter := map[string]any{}
	err = json.Unmarshal(bs, &statsCounter)
	if err != nil {
		ctx.ServerError("MonitorStats", err)
		return
	}
	statsKeys := make([]string, 0, len(statsCounter))
	for k := range statsCounter {
		if statsCounter[k] == nil {
			continue
		}
		statsKeys = append(statsKeys, k)
	}
	sort.Strings(statsKeys)
	ctx.Data["StatsKeys"] = statsKeys
	ctx.Data["StatsCounter"] = statsCounter
	ctx.HTML(http.StatusOK, tplStats)
}
