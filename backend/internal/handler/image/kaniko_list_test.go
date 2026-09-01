package image

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBindKanikoListPageQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/?page=2&page_size=20&search=%20cuda%20&status=Running&status=Finished&sort=-createdAt,image",
		nil,
	)

	request, statuses, err := bindKanikoListPageQuery(ctx)
	if err != nil {
		t.Fatalf("bindKanikoListPageQuery() error = %v", err)
	}
	if request.Page != 2 || request.PageSize != 20 || request.Search != "cuda" {
		t.Fatalf("unexpected request: %+v", request)
	}
	if !reflect.DeepEqual(statuses, []string{"Running", "Finished"}) {
		t.Fatalf("statuses = %v", statuses)
	}
}

func TestBindKanikoListPageQueryRejectsInvalidValues(t *testing.T) {
	tests := []string{
		"page=0",
		"page_size=201",
		"status=Unknown",
		"sort=description",
		"sort=image,image",
	}
	for _, query := range tests {
		t.Run(query, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
			if _, _, err := bindKanikoListPageQuery(ctx); err == nil {
				t.Fatal("bindKanikoListPageQuery() error = nil, want validation error")
			}
		})
	}
}

func TestKanikoListSortClausesAlwaysHaveTieBreaker(t *testing.T) {
	got := kanikoListSortClauses("-createdAt,image")
	want := []string{"kanikos.created_at DESC", "kanikos.image_link ASC", "kanikos.id DESC"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kanikoListSortClauses() = %v, want %v", got, want)
	}
}
