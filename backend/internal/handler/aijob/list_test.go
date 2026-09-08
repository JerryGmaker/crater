package aijob

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/raids-lab/crater/dao/model"
	"github.com/raids-lab/crater/dao/query"
	"github.com/raids-lab/crater/internal/resputil"
)

func setupAIJobListTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AITask{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	query.SetDefault(db)

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}

func createAIJobTestUsers(t *testing.T, db *gorm.DB, names ...string) {
	t.Helper()
	for index, name := range names {
		user := model.User{
			Model:    gorm.Model{ID: uint(index + 1)},
			Name:     name,
			Nickname: "nickname-" + name,
			Role:     model.RoleUser,
			Status:   model.StatusActive,
			Space:    "/tmp/" + name,
		}
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("create test user %q: %v", name, err)
		}
	}
}

func createAITasks(t *testing.T, db *gorm.DB, tasks ...model.AITask) {
	t.Helper()
	for index := range tasks {
		task := tasks[index]
		if task.TaskType == "" {
			task.TaskType = model.EmiasTrainingTask
		}
		if task.Namespace == "" {
			task.Namespace = "test"
		}
		if task.Image == "" {
			task.Image = "test-image"
		}
		if task.ResourceRequest == "" {
			task.ResourceRequest = "{}"
		}
		if err := db.Create(&task).Error; err != nil {
			t.Fatalf("create test AI task %d: %v", task.ID, err)
		}
	}
}

func newAIJobListTestRequest(t *testing.T, page, pageSize int, search string, statuses ...string) aiJobListQuery {
	t.Helper()
	sortClauses, err := aiJobSortClauses("createdAt")
	if err != nil {
		t.Fatalf("build test sort clauses: %v", err)
	}
	return aiJobListQuery{
		Page:        page,
		PageSize:    pageSize,
		Search:      search,
		Statuses:    statuses,
		sortClauses: sortClauses,
	}
}

func aiTaskIDs(items []*model.AITask) []uint {
	ids := make([]uint, len(items))
	for index, item := range items {
		ids[index] = item.ID
	}
	return ids
}

func TestBindAIJobListQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/?page=2&page_size=20&search=%20train%20&days=7&job_type=training&status=Completed&priority=high&profile_status=3&sort=-createdAt,name",
		nil,
	)
	request, err := bindAIJobListQuery(ctx, true)
	if err != nil {
		t.Fatalf("bindAIJobListQuery() error = %v", err)
	}
	if request.Page != 2 || request.PageSize != 20 || request.Search != "train" {
		t.Fatalf("unexpected request: %+v", request)
	}
	if !reflect.DeepEqual(request.sortClauses, []string{"ai_tasks.created_at DESC", "ai_tasks.task_name ASC", "ai_tasks.id DESC"}) {
		t.Fatalf("sortClauses = %v", request.sortClauses)
	}
}

func TestAIJobDatabaseStatusesUseDisplayedValues(t *testing.T) {
	got, includeDeleted := aiJobDatabaseStatuses([]string{"Pending", "Completed", "Aborted", "Deleted"})
	want := []string{"Queueing", "Created", "Pending", "Freed", "Succeeded", "Preempted"}
	if !reflect.DeepEqual(got, want) || !includeDeleted {
		t.Fatalf("aiJobDatabaseStatuses() = %v, %v; want %v, true", got, includeDeleted, want)
	}
}

func TestBindAIJobListQueryRejectsInvalidValues(t *testing.T) {
	tests := []string{
		"page=0",
		"page_size=201",
		"days=0",
		"job_type=unknown",
		"status=Unknown",
		"priority=medium",
		"profile_status=6",
		"sort=resources",
		"sort=name,name",
	}
	for _, rawQuery := range tests {
		t.Run(rawQuery, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
			if _, err := bindAIJobListQuery(ctx, true); err == nil {
				t.Fatal("bindAIJobListQuery() error = nil, want validation error")
			}
		})
	}
}

func TestAIJobSortClausesAlwaysHaveTieBreaker(t *testing.T) {
	got, err := aiJobSortClauses("-priority,profileStatus")
	if err != nil {
		t.Fatalf("aiJobSortClauses() error = %v", err)
	}
	want := []string{"ai_tasks.slo DESC", "ai_tasks.profile_status ASC", "ai_tasks.id DESC"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("aiJobSortClauses() = %v, want %v", got, want)
	}
}

func TestFindAIJobsSearchesAndFiltersBeforePagination(t *testing.T) {
	db := setupAIJobListTestDB(t)
	createAIJobTestUsers(t, db, "alice", "bob")
	createdAt := time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC)
	createAITasks(t, db,
		model.AITask{Model: gorm.Model{ID: 1, CreatedAt: createdAt}, TaskName: "needle one", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
		model.AITask{Model: gorm.Model{ID: 2, CreatedAt: createdAt}, TaskName: "unrelated", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
		model.AITask{Model: gorm.Model{ID: 3, CreatedAt: createdAt}, TaskName: "needle two", UserName: "queue-a", Owner: "bob", Status: model.EmiasTaskSucceededStatus},
		model.AITask{Model: gorm.Model{ID: 4, CreatedAt: createdAt}, TaskName: "needle pending", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskPendingStatus},
	)

	firstRequest := newAIJobListTestRequest(t, 1, 1, "needle", "Completed")
	first, total, err := findAIJobs(t.Context(), aiJobListScope{}, &firstRequest)
	if err != nil {
		t.Fatalf("find first filtered page: %v", err)
	}
	secondRequest := newAIJobListTestRequest(t, 2, 1, "needle", "Completed")
	second, secondTotal, err := findAIJobs(t.Context(), aiJobListScope{}, &secondRequest)
	if err != nil {
		t.Fatalf("find second filtered page: %v", err)
	}

	if total != 2 || secondTotal != 2 {
		t.Fatalf("search and status filter must happen before pagination: first=%d second=%d", total, secondTotal)
	}
	if got := aiTaskIDs(first); !reflect.DeepEqual(got, []uint{3}) {
		t.Fatalf("unexpected first filtered page: %v", got)
	}
	if got := aiTaskIDs(second); !reflect.DeepEqual(got, []uint{1}) {
		t.Fatalf("unexpected second filtered page: %v", got)
	}
}

func TestFindAIJobsMapsDisplayedStatuses(t *testing.T) {
	db := setupAIJobListTestDB(t)
	createAIJobTestUsers(t, db, "alice")
	createdAt := time.Date(2026, time.August, 5, 11, 0, 0, 0, time.UTC)
	createAITasks(t, db,
		model.AITask{Model: gorm.Model{ID: 1, CreatedAt: createdAt}, TaskName: "queueing", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskQueueingStatus},
		model.AITask{Model: gorm.Model{ID: 2, CreatedAt: createdAt}, TaskName: "created", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskCreatedStatus},
		model.AITask{Model: gorm.Model{ID: 3, CreatedAt: createdAt}, TaskName: "pending", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskPendingStatus},
		model.AITask{Model: gorm.Model{ID: 4, CreatedAt: createdAt}, TaskName: "freed", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskFreedStatus},
		model.AITask{Model: gorm.Model{ID: 5, CreatedAt: createdAt}, TaskName: "succeeded", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
		model.AITask{Model: gorm.Model{ID: 6, CreatedAt: createdAt}, TaskName: "preempted", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskPreemptedStatus},
	)

	tests := []struct {
		displayed string
		wantIDs   []uint
	}{
		{displayed: "Pending", wantIDs: []uint{4, 3, 2, 1}},
		{displayed: "Completed", wantIDs: []uint{5}},
		{displayed: "Aborted", wantIDs: []uint{6}},
	}
	for _, test := range tests {
		t.Run(test.displayed, func(t *testing.T) {
			request := newAIJobListTestRequest(t, 1, 20, "", test.displayed)
			items, total, err := findAIJobs(t.Context(), aiJobListScope{}, &request)
			if err != nil {
				t.Fatalf("find %s jobs: %v", test.displayed, err)
			}
			if total != int64(len(test.wantIDs)) || !reflect.DeepEqual(aiTaskIDs(items), test.wantIDs) {
				t.Fatalf("%s result = total %d, ids %v; want total %d, ids %v", test.displayed, total, aiTaskIDs(items), len(test.wantIDs), test.wantIDs)
			}
			for _, item := range items {
				response := convertToAIJobRespWithUser(item, nil)
				if response.Status != test.displayed {
					t.Errorf("task %d response status = %q, want %q", item.ID, response.Status, test.displayed)
				}
			}
		})
	}
}

func TestAIJobFacetsMatchFilteredListCounts(t *testing.T) {
	db := setupAIJobListTestDB(t)
	createAIJobTestUsers(t, db, "alice", "bob")
	createdAt := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	createAITasks(t, db,
		model.AITask{Model: gorm.Model{ID: 1, CreatedAt: createdAt}, TaskName: "needle completed one", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
		model.AITask{Model: gorm.Model{ID: 2, CreatedAt: createdAt}, TaskName: "needle completed two", UserName: "queue-a", Owner: "bob", Status: model.EmiasTaskSucceededStatus},
		model.AITask{Model: gorm.Model{ID: 3, CreatedAt: createdAt}, TaskName: "needle pending", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskPendingStatus},
		model.AITask{Model: gorm.Model{ID: 4, CreatedAt: createdAt}, TaskName: "needle aborted", UserName: "queue-b", Owner: "alice", Status: model.EmiasTaskPreemptedStatus},
		model.AITask{Model: gorm.Model{ID: 5, CreatedAt: createdAt}, TaskName: "noise completed", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
	)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?search=needle", nil)
	(&AIJobMgr{}).listJobFacets(ctx, aiJobListScope{})
	if recorder.Code != http.StatusOK {
		t.Fatalf("list facets status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data resputil.FacetResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode facets response: %v", err)
	}

	statusCounts := make(map[string]int64)
	for _, item := range response.Data.Facets["status"] {
		statusCounts[item.Value] = item.Count
	}
	for _, displayed := range []string{"Pending", "Completed", "Aborted"} {
		request := newAIJobListTestRequest(t, 1, 20, "needle", displayed)
		_, total, err := findAIJobs(t.Context(), aiJobListScope{}, &request)
		if err != nil {
			t.Fatalf("find %s jobs for facet comparison: %v", displayed, err)
		}
		if statusCounts[displayed] != total {
			t.Errorf("facet %s count = %d, filtered list total = %d", displayed, statusCounts[displayed], total)
		}
	}
}

func TestFindAIJobsUsesStableSortAcrossPages(t *testing.T) {
	db := setupAIJobListTestDB(t)
	createAIJobTestUsers(t, db, "alice")
	createdAt := time.Date(2026, time.August, 5, 13, 0, 0, 0, time.UTC)
	createAITasks(t, db,
		model.AITask{Model: gorm.Model{ID: 1, CreatedAt: createdAt}, TaskName: "one", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
		model.AITask{Model: gorm.Model{ID: 2, CreatedAt: createdAt}, TaskName: "two", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
		model.AITask{Model: gorm.Model{ID: 3, CreatedAt: createdAt}, TaskName: "three", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
	)

	firstRequest := newAIJobListTestRequest(t, 1, 2, "")
	first, total, err := findAIJobs(t.Context(), aiJobListScope{}, &firstRequest)
	if err != nil {
		t.Fatalf("find first stable page: %v", err)
	}
	secondRequest := newAIJobListTestRequest(t, 2, 2, "")
	second, secondTotal, err := findAIJobs(t.Context(), aiJobListScope{}, &secondRequest)
	if err != nil {
		t.Fatalf("find second stable page: %v", err)
	}

	if total != 3 || secondTotal != 3 {
		t.Fatalf("unexpected stable-sort totals: first=%d second=%d", total, secondTotal)
	}
	if got := aiTaskIDs(first); !reflect.DeepEqual(got, []uint{3, 2}) {
		t.Fatalf("first page ids = %v, want [3 2]", got)
	}
	if got := aiTaskIDs(second); !reflect.DeepEqual(got, []uint{1}) {
		t.Fatalf("second page ids = %v, want [1]", got)
	}
}

func TestFindAIJobsReturnsEmptyForOutOfRangePage(t *testing.T) {
	db := setupAIJobListTestDB(t)
	createAIJobTestUsers(t, db, "alice")
	createAITasks(t, db,
		model.AITask{Model: gorm.Model{ID: 1}, TaskName: "one", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
		model.AITask{Model: gorm.Model{ID: 2}, TaskName: "two", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
		model.AITask{Model: gorm.Model{ID: 3}, TaskName: "three", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
	)

	request := newAIJobListTestRequest(t, 3, 2, "")
	items, total, err := findAIJobs(t.Context(), aiJobListScope{}, &request)
	if err != nil {
		t.Fatalf("find out-of-range page: %v", err)
	}
	if total != 3 || len(items) != 0 {
		t.Fatalf("out-of-range page = total %d, items %v; want total 3 and no items", total, aiTaskIDs(items))
	}
}

func TestFindAIJobsKeepsUserQueueAndAdminScopesSeparate(t *testing.T) {
	db := setupAIJobListTestDB(t)
	createAIJobTestUsers(t, db, "alice", "bob")
	createAITasks(t, db,
		model.AITask{Model: gorm.Model{ID: 1}, TaskName: "alice queue a", UserName: "queue-a", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
		model.AITask{Model: gorm.Model{ID: 2}, TaskName: "bob queue a", UserName: "queue-a", Owner: "bob", Status: model.EmiasTaskSucceededStatus},
		model.AITask{Model: gorm.Model{ID: 3}, TaskName: "alice queue b", UserName: "queue-b", Owner: "alice", Status: model.EmiasTaskSucceededStatus},
	)

	request := newAIJobListTestRequest(t, 1, 20, "")
	request.sortClauses = []string{"ai_tasks.id ASC"}
	all, allTotal, err := findAIJobs(t.Context(), aiJobListScope{}, &request)
	if err != nil {
		t.Fatalf("find admin scope: %v", err)
	}
	queue := "queue-a"
	queueItems, queueTotal, err := findAIJobs(t.Context(), aiJobListScope{queue: &queue}, &request)
	if err != nil {
		t.Fatalf("find user queue scope: %v", err)
	}
	owner := "alice"
	ownerItems, ownerTotal, err := findAIJobs(t.Context(), aiJobListScope{owner: &owner}, &request)
	if err != nil {
		t.Fatalf("find user owner scope: %v", err)
	}

	if allTotal != 3 || !reflect.DeepEqual(aiTaskIDs(all), []uint{1, 2, 3}) {
		t.Fatalf("admin scope = total %d, ids %v; want total 3, ids [1 2 3]", allTotal, aiTaskIDs(all))
	}
	if queueTotal != 2 || !reflect.DeepEqual(aiTaskIDs(queueItems), []uint{1, 2}) {
		t.Fatalf("queue scope = total %d, ids %v; want total 2, ids [1 2]", queueTotal, aiTaskIDs(queueItems))
	}
	if ownerTotal != 2 || !reflect.DeepEqual(aiTaskIDs(ownerItems), []uint{1, 3}) {
		t.Fatalf("owner scope = total %d, ids %v; want total 2, ids [1 3]", ownerTotal, aiTaskIDs(ownerItems))
	}
}
