package aijob

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

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
