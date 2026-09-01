package aijob

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/raids-lab/crater/dao/model"
	"github.com/raids-lab/crater/dao/query"
	"github.com/raids-lab/crater/internal/bizerr"
	"github.com/raids-lab/crater/internal/resputil"
	interutil "github.com/raids-lab/crater/internal/util"
)

const aiJobMaxSearchRunes = 128

type aiJobListScope struct {
	queue *string
	owner *string
}

type aiJobListQuery struct {
	Page            int      `form:"page,default=1" binding:"min=1"`
	PageSize        int      `form:"page_size,default=10" binding:"min=1,max=200"`
	Sort            string   `form:"sort"`
	Search          string   `form:"search"`
	Days            *int     `form:"days" binding:"omitempty,eq=-1|gt=0"`
	JobTypes        []string `form:"job_type" binding:"max=20,dive,required"`
	Statuses        []string `form:"status" binding:"max=20,dive,required"`
	Priorities      []string `form:"priority" binding:"max=2,dive,oneof=high low"`
	ProfileStatuses []uint   `form:"profile_status" binding:"max=6,dive,max=5"`
	sortClauses     []string
}

func bindAIJobListQuery(c *gin.Context, withSort bool) (aiJobListQuery, error) {
	var request aiJobListQuery
	if err := c.ShouldBindQuery(&request); err != nil {
		return aiJobListQuery{}, bizerr.BadRequest.ParameterError.Wrap(err, "invalid AI job list query")
	}
	request.Search = strings.TrimSpace(request.Search)
	if utf8.RuneCountInString(request.Search) > aiJobMaxSearchRunes {
		return aiJobListQuery{}, bizerr.BadRequest.ParameterError.New("search accepts at most 128 characters")
	}
	if request.Page > int(^uint(0)>>1)/request.PageSize {
		return aiJobListQuery{}, bizerr.BadRequest.ParameterError.New("page is too large for page_size")
	}
	if err := validateAIJobEnums(&request); err != nil {
		return aiJobListQuery{}, err
	}
	if withSort {
		clauses, err := aiJobSortClauses(request.Sort)
		if err != nil {
			return aiJobListQuery{}, err
		}
		request.sortClauses = clauses
	}
	return request, nil
}

func validateAIJobEnums(request *aiJobListQuery) error {
	jobTypes := map[string]struct{}{
		model.EmiasTrainingTask: {}, model.EmiasJupyterTask: {}, model.EmiasDebuggingTask: {},
		model.EmiasInferenceTask: {}, model.EmiasExperimentTask: {},
	}
	for _, value := range request.JobTypes {
		if _, ok := jobTypes[value]; !ok {
			return bizerr.BadRequest.ParameterError.New("unsupported job_type " + strconv.Quote(value))
		}
	}
	statuses := map[string]struct{}{
		"Pending": {}, "Running": {}, "Completed": {}, "Failed": {},
		"Aborted": {}, "Deleted": {},
	}
	for _, value := range request.Statuses {
		if _, ok := statuses[value]; !ok {
			return bizerr.BadRequest.ParameterError.New("unsupported status " + strconv.Quote(value))
		}
	}
	return nil
}

func aiJobSortClauses(raw string) ([]string, error) {
	fields := map[string]string{
		"id": "ai_tasks.id", "name": "ai_tasks.task_name", "jobName": "ai_tasks.id",
		"owner": "users.nickname", "queue": "ai_tasks.username", "jobType": "ai_tasks.task_type",
		"status": "ai_tasks.status", "priority": "ai_tasks.slo",
		"profileStatus": "ai_tasks.profile_status", "createdAt": "ai_tasks.created_at",
		"startedAt": "ai_tasks.started_at", "completedAt": "ai_tasks.finish_at",
	}
	if raw == "" {
		return []string{"ai_tasks.created_at DESC", "ai_tasks.id DESC"}, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 3 {
		return nil, bizerr.BadRequest.ParameterError.New("sort accepts at most 3 fields")
	}
	seen := make(map[string]struct{}, len(parts)+1)
	clauses := make([]string, 0, len(parts)+1)
	for _, part := range parts {
		descending := strings.HasPrefix(part, "-")
		name := strings.TrimPrefix(part, "-")
		column, ok := fields[name]
		if !ok {
			return nil, bizerr.BadRequest.ParameterError.New("unsupported sort field " + strconv.Quote(name))
		}
		if _, ok := seen[name]; ok {
			return nil, bizerr.BadRequest.ParameterError.New("duplicate sort field " + strconv.Quote(name))
		}
		seen[name] = struct{}{}
		direction := " ASC"
		if descending {
			direction = " DESC"
		}
		clauses = append(clauses, column+direction)
	}
	if _, ok := seen["id"]; !ok {
		clauses = append(clauses, "ai_tasks.id DESC")
	}
	return clauses, nil
}

func applyAIJobFilters(
	ctx context.Context,
	scope aiJobListScope,
	request *aiJobListQuery,
) *gorm.DB {
	db := query.AITask.WithContext(ctx).UnderlyingDB().
		Model(&model.AITask{}).
		Select("ai_tasks.*").
		Joins("LEFT JOIN users ON users.name = ai_tasks.owner")
	if scope.queue != nil {
		db = db.Where("ai_tasks.username = ?", *scope.queue)
	}
	if scope.owner != nil {
		db = db.Where("ai_tasks.owner = ?", *scope.owner)
	}
	if len(request.JobTypes) > 0 {
		db = db.Where("ai_tasks.task_type IN ?", request.JobTypes)
	}
	if len(request.Statuses) > 0 {
		dbStatuses, includeDeleted := aiJobDatabaseStatuses(request.Statuses)
		switch {
		case includeDeleted && len(dbStatuses) > 0:
			db = db.Where(
				"ai_tasks.is_deleted = TRUE OR (ai_tasks.is_deleted = FALSE AND ai_tasks.status IN ?)",
				dbStatuses,
			)
		case includeDeleted:
			db = db.Where("ai_tasks.is_deleted = TRUE")
		default:
			db = db.Where("ai_tasks.is_deleted = FALSE AND ai_tasks.status IN ?", dbStatuses)
		}
	}
	if len(request.Priorities) == 1 {
		if request.Priorities[0] == "high" {
			db = db.Where("ai_tasks.slo > 0")
		} else {
			db = db.Where("ai_tasks.slo = 0")
		}
	}
	if len(request.ProfileStatuses) > 0 {
		includeSkipped := false
		for _, status := range request.ProfileStatuses {
			includeSkipped = includeSkipped || status == model.EmiasProfileSkipped
		}
		if includeSkipped {
			db = db.Where(
				"(ai_tasks.profile_status IN ? AND (ai_tasks.is_deleted = FALSE OR ai_tasks.profile_status = ?)) OR "+
					"(ai_tasks.is_deleted = TRUE AND ai_tasks.profile_status <> ?)",
				request.ProfileStatuses, model.EmiasProfileFinish, model.EmiasProfileFinish,
			)
		} else {
			db = db.Where(
				"ai_tasks.profile_status IN ? AND (ai_tasks.is_deleted = FALSE OR ai_tasks.profile_status = ?)",
				request.ProfileStatuses, model.EmiasProfileFinish,
			)
		}
	}
	if request.Days != nil && *request.Days != -1 {
		db = db.Where("ai_tasks.created_at >= ?", time.Now().AddDate(0, 0, -*request.Days))
	}
	if request.Search != "" {
		pattern := interutil.ContainsPattern(request.Search)
		db = db.Where(
			"LOWER(ai_tasks.task_name) LIKE ? OR LOWER(ai_tasks.owner) LIKE ? OR "+
				"LOWER(ai_tasks.username) LIKE ? OR LOWER(COALESCE(users.nickname, '')) LIKE ?",
			pattern, pattern, pattern, pattern,
		)
	}
	return db
}

func aiJobDatabaseStatuses(statuses []string) ([]string, bool) {
	values := make([]string, 0, len(statuses)+2)
	seen := make(map[string]struct{}, len(statuses)+2)
	includeDeleted := false
	appendStatus := func(value string) {
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	for _, status := range statuses {
		switch status {
		case "Pending":
			appendStatus(model.EmiasTaskQueueingStatus)
			appendStatus(model.EmiasTaskCreatedStatus)
			appendStatus(model.EmiasTaskPendingStatus)
			appendStatus(model.EmiasTaskFreedStatus)
		case "Completed":
			appendStatus(model.EmiasTaskSucceededStatus)
		case "Aborted":
			appendStatus(model.EmiasTaskPreemptedStatus)
		case "Deleted":
			includeDeleted = true
		default:
			appendStatus(status)
		}
	}
	return values, includeDeleted
}

func findAIJobs(
	ctx context.Context,
	scope aiJobListScope,
	request *aiJobListQuery,
) ([]*model.AITask, int64, error) {
	db := applyAIJobFilters(ctx, scope, request)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	for _, clause := range request.sortClauses {
		db = db.Order(clause)
	}
	items := make([]*model.AITask, 0, request.PageSize)
	err := db.Offset((request.Page - 1) * request.PageSize).Limit(request.PageSize).Find(&items).Error
	return items, total, err
}

func (mgr *AIJobMgr) listJobPage(c *gin.Context, scope aiJobListScope) {
	request, err := bindAIJobListQuery(c, true)
	if err != nil {
		resputil.HandleError(c, err)
		return
	}
	items, total, err := findAIJobs(c, scope, &request)
	if err != nil {
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "list AI jobs failed"))
		return
	}
	responses, err := convertAIJobPage(c, items)
	if err != nil {
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "load AI job users failed"))
		return
	}
	resputil.Success(c, resputil.NewPage(responses, total, request.Page, request.PageSize))
}

func convertAIJobPage(ctx context.Context, items []*model.AITask) ([]AIJobResp, error) {
	owners := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, ok := seen[item.Owner]; !ok {
			seen[item.Owner] = struct{}{}
			owners = append(owners, item.Owner)
		}
	}
	users := make([]*model.User, 0, len(owners))
	if len(owners) > 0 {
		u := query.User
		if err := u.WithContext(ctx).Where(u.Name.In(owners...)).Scan(&users); err != nil {
			return nil, err
		}
	}
	userByName := make(map[string]*model.User, len(users))
	for _, user := range users {
		userByName[user.Name] = user
	}
	responses := make([]AIJobResp, len(items))
	for index, item := range items {
		responses[index] = convertToAIJobRespWithUser(item, userByName[item.Owner])
	}
	return responses, nil
}

func (mgr *AIJobMgr) ListSelfJobPage(c *gin.Context) {
	token := interutil.GetToken(c)
	mgr.listJobPage(c, aiJobListScope{queue: &token.AccountName})
}

func (mgr *AIJobMgr) ListAllJobPage(c *gin.Context) {
	mgr.listJobPage(c, aiJobListScope{})
}

func (mgr *AIJobMgr) ListUserJobPage(c *gin.Context) {
	owner := strings.TrimSpace(c.Param("username"))
	if owner == "" {
		resputil.HandleError(c, bizerr.BadRequest.MissingParameter.New("username is required"))
		return
	}
	mgr.listJobPage(c, aiJobListScope{owner: &owner})
}

func (mgr *AIJobMgr) listJobFacets(c *gin.Context, scope aiJobListScope) {
	request, err := bindAIJobListQuery(c, false)
	if err != nil {
		resputil.HandleError(c, err)
		return
	}
	facets := make(map[string][]resputil.FacetItem, 4)
	queries := []struct {
		name       string
		expression string
		clear      func(*aiJobListQuery)
	}{
		{"job_type", "ai_tasks.task_type", func(q *aiJobListQuery) { q.JobTypes = nil }},
		{"status", "CASE WHEN ai_tasks.is_deleted THEN 'Deleted' WHEN ai_tasks.status IN ('Queueing', 'Created', 'Freed') THEN 'Pending' WHEN ai_tasks.status = 'Succeeded' THEN 'Completed' WHEN ai_tasks.status = 'Preempted' THEN 'Aborted' ELSE ai_tasks.status END", func(q *aiJobListQuery) { q.Statuses = nil }},
		{"priority", "CASE WHEN ai_tasks.slo > 0 THEN 'high' ELSE 'low' END", func(q *aiJobListQuery) { q.Priorities = nil }},
		{"profile_status", "CASE WHEN ai_tasks.is_deleted AND ai_tasks.profile_status <> 3 THEN '5' ELSE CAST(ai_tasks.profile_status AS TEXT) END", func(q *aiJobListQuery) { q.ProfileStatuses = nil }},
	}
	for _, facet := range queries {
		facetRequest := request
		facet.clear(&facetRequest)
		rows := make([]resputil.FacetItem, 0)
		err = applyAIJobFilters(c, scope, &facetRequest).
			Select(facet.expression + " AS value, COUNT(ai_tasks.id) AS count").
			Group(facet.expression).
			Order("count DESC, value ASC").
			Scan(&rows).Error
		if err != nil {
			resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, fmt.Sprintf("load %s facet failed", facet.name)))
			return
		}
		facets[facet.name] = rows
	}
	resputil.Success(c, resputil.FacetResponse{Facets: facets})
}

func (mgr *AIJobMgr) ListSelfJobFacets(c *gin.Context) {
	token := interutil.GetToken(c)
	mgr.listJobFacets(c, aiJobListScope{queue: &token.AccountName})
}

func (mgr *AIJobMgr) ListAllJobFacets(c *gin.Context) {
	mgr.listJobFacets(c, aiJobListScope{})
}

func (mgr *AIJobMgr) ListUserJobFacets(c *gin.Context) {
	owner := strings.TrimSpace(c.Param("username"))
	if owner == "" {
		resputil.HandleError(c, bizerr.BadRequest.MissingParameter.New("username is required"))
		return
	}
	mgr.listJobFacets(c, aiJobListScope{owner: &owner})
}
