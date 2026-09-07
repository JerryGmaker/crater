package image

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBindGetKanikoRequestSupportsIDAndLegacyName(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantID   uint
		wantName string
	}{
		{name: "stable id", query: "id=42", wantID: 42},
		{name: "legacy name", query: "name=example-build", wantName: "example-build"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil)

			var request GetKanikoRequest
			if err := ctx.ShouldBindQuery(&request); err != nil {
				t.Fatalf("ShouldBindQuery() error = %v", err)
			}
			if request.ID != tt.wantID || request.ImagePackName != tt.wantName {
				t.Fatalf("request = %+v, want id=%d name=%q", request, tt.wantID, tt.wantName)
			}
		})
	}
}
