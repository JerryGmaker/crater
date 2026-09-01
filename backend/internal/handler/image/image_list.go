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

type imageListPageQuery struct {
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=10" binding:"min=1,max=200"`
	Search   string `form:"search"`
	Sort     string `form:"sort"`
}

var imageListSortFields = map[string]string{
	"image":            "images.image_link",
	"userInfo":         "users.name",
	"createdAt":        "images.created_at",
	"imageShareStatus": "",
}

var imageListShareStatuses = map[string]struct{}{
	string(model.Public):       {},
	string(model.Private):      {},
	string(model.UserShare):    {},
	string(model.AccountShare): {},
}

func validateImageListSort(sortValue string) error {
	parts := strings.Split(sortValue, ",")
	if len(parts) > 3 {
		return bizerr.BadRequest.ParameterError.New("sort accepts at most 3 fields")
	}
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		name := strings.TrimPrefix(part, "-")
		if _, ok := imageListSortFields[name]; !ok {
			return bizerr.BadRequest.ParameterError.New(fmt.Sprintf("unsupported sort field %q", name))
		}
		if _, ok := seen[name]; ok {
			return bizerr.BadRequest.ParameterError.New(fmt.Sprintf("duplicate sort field %q", name))
		}
		seen[name] = struct{}{}
	}
	return nil
}

func imageListSortClauses(sortValue, visibilityExpression string) []string {
	clauses := make([]string, 0, 4)
	for _, part := range strings.Split(sortValue, ",") {
		name := strings.TrimPrefix(part, "-")
		field := imageListSortFields[name]
		if name == "imageShareStatus" {
			field = visibilityExpression
		}
		direction := "ASC"
		if strings.HasPrefix(part, "-") {
			direction = "DESC"
		}
		clauses = append(clauses, field+" "+direction)
	}
	return append(clauses, "images.id DESC")
}

func bindImageListPageQuery(c *gin.Context) (imageListPageQuery, []string, error) {
	var request imageListPageQuery
	if err := c.ShouldBindQuery(&request); err != nil {
		return imageListPageQuery{}, nil, bizerr.BadRequest.ParameterError.Wrap(
			err,
			"invalid image list query",
		)
	}
	request.Search = strings.TrimSpace(request.Search)
	if utf8.RuneCountInString(request.Search) > 128 {
		return imageListPageQuery{}, nil, bizerr.BadRequest.ParameterError.New(
			"search accepts at most 128 characters",
		)
	}
	if request.Page > int(^uint(0)>>1)/request.PageSize {
		return imageListPageQuery{}, nil, bizerr.BadRequest.ParameterError.New(
			"page is too large for page_size",
		)
	}
	if request.Sort == "" {
		request.Sort = "-createdAt"
	}
	if err := validateImageListSort(request.Sort); err != nil {
		return imageListPageQuery{}, nil, err
	}

	shareStatuses := c.QueryArray("imageShareStatus")
	for _, status := range shareStatuses {
		if _, ok := imageListShareStatuses[status]; !ok {
			return imageListPageQuery{}, nil, bizerr.BadRequest.ParameterError.New(
				fmt.Sprintf("unsupported image visibility %q", status),
			)
		}
	}
	return request, shareStatuses, nil
}

func imageVisibilityExpression(userID, accountID uint, isAdmin bool) string {
	publicCondition := fmt.Sprintf(
		"(images.is_public = TRUE OR EXISTS (SELECT 1 FROM image_accounts ia_public "+
			"WHERE ia_public.image_id = images.id AND ia_public.account_id = %d "+
			"AND ia_public.deleted_at IS NULL))",
		model.DefaultAccountID,
	)
	if isAdmin {
		return fmt.Sprintf(
			"CASE WHEN %s THEN '%s' ELSE '%s' END",
			publicCondition,
			model.Public,
			model.Private,
		)
	}
	return fmt.Sprintf(
		"CASE "+
			"WHEN %s THEN '%s' "+
			"WHEN EXISTS (SELECT 1 FROM image_accounts ia_account WHERE ia_account.image_id = images.id "+
			"AND ia_account.account_id = %d AND ia_account.deleted_at IS NULL) THEN '%s' "+
			"WHEN images.user_id = %d THEN '%s' "+
			"WHEN EXISTS (SELECT 1 FROM image_users iu WHERE iu.image_id = images.id "+
			"AND iu.user_id = %d AND iu.deleted_at IS NULL) THEN '%s' "+
			"ELSE NULL END",
		publicCondition,
		model.Public,
		accountID,
		model.AccountShare,
		userID,
		model.Private,
		userID,
		model.UserShare,
	)
}

type visibleImageRow struct {
	ID               uint                 `gorm:"column:id"`
	ImageShareStatus model.ImageShareType `gorm:"column:image_share_status"`
}

func (mgr *ImagePackMgr) listImagePage(c *gin.Context, isAdmin bool) {
	request, shareStatuses, err := bindImageListPageQuery(c)
	if err != nil {
		resputil.HandleError(c, err)
		return
	}
	token := util.GetToken(c)
	visibilityExpression := imageVisibilityExpression(token.UserID, token.AccountID, isAdmin)

	db := query.GetDB().WithContext(c).
		Model(&model.Image{}).
		Joins("LEFT JOIN users ON users.id = images.user_id").
		Where("images.image_link <> ''")
	if !isAdmin {
		db = db.Where("(" + visibilityExpression + ") IS NOT NULL")
	}
	if len(shareStatuses) > 0 {
		db = db.Where("("+visibilityExpression+") IN ?", shareStatuses)
	}
	if request.Search != "" {
		pattern := util.ContainsPattern(request.Search)
		db = db.Where(
			"(LOWER(images.image_link) LIKE ? ESCAPE '\\' OR "+
				"LOWER(COALESCE(images.description, '')) LIKE ? ESCAPE '\\' OR "+
				"LOWER(COALESCE(images.image_pack_name, '')) LIKE ? ESCAPE '\\' OR "+
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
		klog.Errorf("failed to count visible images: %v", err)
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "count visible images failed"))
		return
	}
	for _, clause := range imageListSortClauses(request.Sort, visibilityExpression) {
		db = db.Order(clause)
	}
	rows := make([]visibleImageRow, 0, request.PageSize)
	if err := db.Select("images.id, " + visibilityExpression + " AS image_share_status").
		Offset((request.Page - 1) * request.PageSize).
		Limit(request.PageSize).
		Scan(&rows).Error; err != nil {
		klog.Errorf("failed to list visible image ids: %v", err)
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "list visible images failed"))
		return
	}

	items := make([]*ImageInfo, 0, len(rows))
	if len(rows) > 0 {
		ids := make([]uint, len(rows))
		for i, row := range rows {
			ids[i] = row.ID
		}
		imageQuery := query.Image
		images, err := imageQuery.WithContext(c).
			Preload(imageQuery.User).
			Where(imageQuery.ID.In(ids...)).
			Find()
		if err != nil {
			klog.Errorf("failed to load visible images: %v", err)
			resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "load visible images failed"))
			return
		}
		imagesByID := make(map[uint]*model.Image, len(images))
		for _, image := range images {
			imagesByID[image.ID] = image
		}
		for _, row := range rows {
			if image := imagesByID[row.ID]; image != nil {
				items = append(items, mgr.imageInfoFromModel(image, row.ImageShareStatus))
			}
		}
	}

	resputil.Success(c, resputil.NewPage(items, total, request.Page, request.PageSize))
}

// UserListImagePage returns images visible to the current user using the shared page protocol.
//
//	@Summary	分页获取当前用户可见镜像
//	@Tags		ImagePack
//	@Produce	json
//	@Security	Bearer
//	@Param		page				query	int		false	"Page number"
//	@Param		page_size			query	int		false	"Page size, 1-200"
//	@Param		search				query	string	false	"Search image, description, or creator"
//	@Param		imageShareStatus	query	string	false	"Filter by visibility; repeatable"
//	@Param		sort				query	string	false	"Sort fields"
//	@Success	200					{object}	resputil.Response[resputil.Page[ImageInfo]]
//	@Router		/v1/images/image/page [get]
func (mgr *ImagePackMgr) UserListImagePage(c *gin.Context) {
	mgr.listImagePage(c, false)
}

// AdminListImagePage returns all images using the shared page protocol.
//
//	@Summary	分页获取全部镜像
//	@Tags		ImagePack
//	@Produce	json
//	@Security	Bearer
//	@Param		page				query	int		false	"Page number"
//	@Param		page_size			query	int		false	"Page size, 1-200"
//	@Param		search				query	string	false	"Search image, description, or creator"
//	@Param		imageShareStatus	query	string	false	"Filter by visibility; repeatable"
//	@Param		sort				query	string	false	"Sort fields"
//	@Success	200					{object}	resputil.Response[resputil.Page[ImageInfo]]
//	@Router		/v1/admin/images/image/page [get]
func (mgr *ImagePackMgr) AdminListImagePage(c *gin.Context) {
	mgr.listImagePage(c, true)
}
