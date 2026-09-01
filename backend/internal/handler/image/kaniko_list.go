package image

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/raids-lab/crater/dao/model"
	"github.com/raids-lab/crater/dao/query"
	"github.com/raids-lab/crater/internal/bizerr"
	"github.com/raids-lab/crater/internal/resputil"
	"github.com/raids-lab/crater/internal/util"
)

type kanikoListPageQuery struct {
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=10" binding:"min=1,max=200"`
	Search   string `form:"search"`
	Sort     string `form:"sort"`
}

var kanikoListSortFields = map[string]string{
	"image":       "kanikos.image_link",
	"userInfo":    "users.name",
	"createdAt":   "kanikos.created_at",
	"status":      "kanikos.status",
	"size":        "kanikos.size",
	"buildSource": "kanikos.build_source",
}

var kanikoListStatuses = map[string]struct{}{
	string(model.BuildJobInitial):  {},
	string(model.BuildJobPending):  {},
	string(model.BuildJobRunning):  {},
	string(model.BuildJobFinished): {},
	string(model.BuildJobFailed):   {},
	string(model.BuildJobCanceled): {},
}

func validateKanikoListSort(sortValue string) error {
	parts := strings.Split(sortValue, ",")
	if len(parts) > 3 {
		return bizerr.BadRequest.ParameterError.New("sort accepts at most 3 fields")
	}
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		name := strings.TrimPrefix(part, "-")
		if _, ok := kanikoListSortFields[name]; !ok {
			return bizerr.BadRequest.ParameterError.New(fmt.Sprintf("unsupported sort field %q", name))
		}
		if _, ok := seen[name]; ok {
			return bizerr.BadRequest.ParameterError.New(fmt.Sprintf("duplicate sort field %q", name))
		}
		seen[name] = struct{}{}
	}
	return nil
}

func kanikoListSortClauses(sortValue string) []string {
	clauses := make([]string, 0, 4)
	for _, part := range strings.Split(sortValue, ",") {
		name := strings.TrimPrefix(part, "-")
		direction := "ASC"
		if strings.HasPrefix(part, "-") {
			direction = "DESC"
		}
		clauses = append(clauses, kanikoListSortFields[name]+" "+direction)
	}
	return append(clauses, "kanikos.id DESC")
}

func bindKanikoListPageQuery(c *gin.Context) (kanikoListPageQuery, []string, error) {
	var request kanikoListPageQuery
	if err := c.ShouldBindQuery(&request); err != nil {
		return kanikoListPageQuery{}, nil, bizerr.BadRequest.ParameterError.Wrap(
			err,
			"invalid image build list query",
		)
	}
	request.Search = strings.TrimSpace(request.Search)
	if utf8.RuneCountInString(request.Search) > 128 {
		return kanikoListPageQuery{}, nil, bizerr.BadRequest.ParameterError.New(
			"search accepts at most 128 characters",
		)
	}
	if request.Page > int(^uint(0)>>1)/request.PageSize {
		return kanikoListPageQuery{}, nil, bizerr.BadRequest.ParameterError.New(
			"page is too large for page_size",
		)
	}
	if request.Sort == "" {
		request.Sort = "-createdAt"
	}
	if err := validateKanikoListSort(request.Sort); err != nil {
		return kanikoListPageQuery{}, nil, err
	}

	statuses := c.QueryArray("status")
	for _, status := range statuses {
		if _, ok := kanikoListStatuses[status]; !ok {
			return kanikoListPageQuery{}, nil, bizerr.BadRequest.ParameterError.New(
				fmt.Sprintf("unsupported image build status %q", status),
			)
		}
	}
	return request, statuses, nil
}

func (mgr *ImagePackMgr) listKanikoPage(c *gin.Context, userID *uint) {
	request, statuses, err := bindKanikoListPageQuery(c)
	if err != nil {
		resputil.HandleError(c, err)
		return
	}

	db := query.GetDB().WithContext(c).
		Model(&model.Kaniko{}).
		Joins("LEFT JOIN users ON users.id = kanikos.user_id")
	if userID != nil {
		db = db.Where("kanikos.user_id = ?", *userID)
	}
	if len(statuses) > 0 {
		db = db.Where("kanikos.status IN ?", statuses)
	}
	if request.Search != "" {
		pattern := util.ContainsPattern(request.Search)
		db = db.Where(
			"(LOWER(kanikos.image_link) LIKE ? ESCAPE '\\' OR "+
				"LOWER(kanikos.image_pack_name) LIKE ? ESCAPE '\\' OR "+
				"LOWER(COALESCE(kanikos.description, '')) LIKE ? ESCAPE '\\' OR "+
				"LOWER(users.name) LIKE ? ESCAPE '\\' OR LOWER(users.nickname) LIKE ? ESCAPE '\\')",
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
		)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		klog.Errorf("failed to count image build records: %v", err)
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "count image build records failed"))
		return
	}
	for _, clause := range kanikoListSortClauses(request.Sort) {
		db = db.Order(clause)
	}
	kanikos := make([]*model.Kaniko, 0, request.PageSize)
	if err := db.Preload("User").
		Offset((request.Page - 1) * request.PageSize).
		Limit(request.PageSize).
		Find(&kanikos).Error; err != nil {
		klog.Errorf("failed to list image build records: %v", err)
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "list image build records failed"))
		return
	}

	resputil.Success(c, resputil.NewPage(mgr.generateKanikoInfos(kanikos), total, request.Page, request.PageSize))
}

// UserListKanikoPage returns the current user's image build records using the shared page protocol.
//
//	@Summary	分页获取当前用户的镜像制作记录
//	@Tags		ImagePack
//	@Produce	json
//	@Security	Bearer
//	@Param		page		query	int		false	"Page number"
//	@Param		page_size	query	int		false	"Page size, 1-200"
//	@Param		search		query	string	false	"Search image, description, or creator"
//	@Param		status		query	string	false	"Filter by status; repeatable"
//	@Param		sort		query	string	false	"Sort fields"
//	@Success	200			{object}	resputil.Response[resputil.Page[KanikoInfo]]
//	@Router		/v1/images/kaniko/page [get]
func (mgr *ImagePackMgr) UserListKanikoPage(c *gin.Context) {
	userID := util.GetToken(c).UserID
	mgr.listKanikoPage(c, &userID)
}

// AdminListKanikoPage returns all image build records using the shared page protocol.
//
//	@Summary	分页获取全部镜像制作记录
//	@Tags		ImagePack
//	@Produce	json
//	@Security	Bearer
//	@Param		page		query	int		false	"Page number"
//	@Param		page_size	query	int		false	"Page size, 1-200"
//	@Param		search		query	string	false	"Search image, description, or creator"
//	@Param		status		query	string	false	"Filter by status; repeatable"
//	@Param		sort		query	string	false	"Sort fields"
//	@Success	200			{object}	resputil.Response[resputil.Page[KanikoInfo]]
//	@Router		/v1/admin/images/kaniko/page [get]
func (mgr *ImagePackMgr) AdminListKanikoPage(c *gin.Context) {
	mgr.listKanikoPage(c, nil)
}
