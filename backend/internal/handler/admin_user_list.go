package handler

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/raids-lab/crater/dao/model"
	"github.com/raids-lab/crater/dao/query"
	"github.com/raids-lab/crater/internal/bizerr"
	"github.com/raids-lab/crater/internal/resputil"
	"github.com/raids-lab/crater/internal/service"
	"github.com/raids-lab/crater/internal/util"
	"github.com/raids-lab/crater/pkg/utils"
)

type adminUserPageQuery struct {
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=10" binding:"min=1,max=200"`
	Search   string `form:"search"`
	Sort     string `form:"sort"`
}

var adminUserSortFields = map[string]string{
	"name":      "name",
	"role":      "role",
	"status":    "status",
	"createdAt": "created_at",
	"updatedAt": "updated_at",
}

func validateAdminUserSort(sortValue string) error {
	parts := strings.Split(sortValue, ",")
	if len(parts) > 3 {
		return bizerr.BadRequest.ParameterError.New("sort accepts at most 3 fields")
	}
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		name := strings.TrimPrefix(part, "-")
		if _, ok := adminUserSortFields[name]; !ok {
			return bizerr.BadRequest.ParameterError.New(fmt.Sprintf("unsupported sort field %q", name))
		}
		if _, ok := seen[name]; ok {
			return bizerr.BadRequest.ParameterError.New(fmt.Sprintf("duplicate sort field %q", name))
		}
		seen[name] = struct{}{}
	}
	return nil
}

func adminUserSortClauses(sortValue string) []string {
	clauses := make([]string, 0, 4)
	for _, part := range strings.Split(sortValue, ",") {
		name := strings.TrimPrefix(part, "-")
		direction := "ASC"
		if strings.HasPrefix(part, "-") {
			direction = "DESC"
		}
		clauses = append(clauses, adminUserSortFields[name]+" "+direction)
	}
	clauses = append(clauses, "id DESC")
	return clauses
}

func bindAdminUserPageQuery(c *gin.Context) (adminUserPageQuery, error) {
	var request adminUserPageQuery
	if err := c.ShouldBindQuery(&request); err != nil {
		return adminUserPageQuery{}, bizerr.BadRequest.ParameterError.Wrap(err, "invalid user list query")
	}
	request.Search = strings.TrimSpace(request.Search)
	if utf8.RuneCountInString(request.Search) > 128 {
		return adminUserPageQuery{}, bizerr.BadRequest.ParameterError.New(
			"search accepts at most 128 characters",
		)
	}
	if request.Page > int(^uint(0)>>1)/request.PageSize {
		return adminUserPageQuery{}, bizerr.BadRequest.ParameterError.New("page is too large for page_size")
	}
	if request.Sort == "" {
		request.Sort = "-createdAt"
	}
	if err := validateAdminUserSort(request.Sort); err != nil {
		return adminUserPageQuery{}, err
	}
	return request, nil
}

func parseAdminUserIntFilters(c *gin.Context, key string, min, max int) ([]int, error) {
	values := c.QueryArray(key)
	result := make([]int, 0, len(values))
	for _, value := range values {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < min || parsed > max {
			return nil, bizerr.BadRequest.ParameterError.New(fmt.Sprintf("invalid %s filter %q", key, value))
		}
		result = append(result, parsed)
	}
	return result, nil
}

func parseAdminUserBannedFilters(c *gin.Context) ([]bool, error) {
	values := c.QueryArray("banned")
	result := make([]bool, 0, len(values))
	for _, value := range values {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, bizerr.BadRequest.ParameterError.New(fmt.Sprintf("invalid banned filter %q", value))
		}
		result = append(result, parsed)
	}
	return result, nil
}

// ListUserPage returns administrator-visible users using the shared page protocol.
//
//	@Summary	管理员获取用户分页
//	@Description	按统一分页协议获取用户列表，搜索、筛选、排序和分页在数据库中执行
//	@Tags		User
//	@Produce	json
//	@Security	Bearer
//	@Param		page	query	int	false	"Page number"
//	@Param		page_size	query	int	false	"Page size, 1-200"
//	@Param		search	query	string	false	"Search username or nickname"
//	@Param		role	query	int	false	"Filter by role; repeatable"
//	@Param		status	query	int	false	"Filter by status; repeatable"
//	@Param		banned	query	bool	false	"Filter by current ban status; repeatable"
//	@Param		sort	query	string	false	"Sort fields"
//	@Success	200	{object}	resputil.Response[resputil.Page[UserResp]]
//	@Router		/v1/admin/users/page [get]
func (mgr *UserMgr) ListUserPage(c *gin.Context) {
	request, err := bindAdminUserPageQuery(c)
	if err != nil {
		resputil.HandleError(c, err)
		return
	}
	roles, err := parseAdminUserIntFilters(c, "role", 1, 3)
	if err != nil {
		resputil.HandleError(c, err)
		return
	}
	statuses, err := parseAdminUserIntFilters(c, "status", 1, 3)
	if err != nil {
		resputil.HandleError(c, err)
		return
	}
	bannedFilters, err := parseAdminUserBannedFilters(c)
	if err != nil {
		resputil.HandleError(c, err)
		return
	}

	db := query.User.WithContext(c).UnderlyingDB().Model(&model.User{})
	if request.Search != "" {
		pattern := util.ContainsPattern(request.Search)
		db = db.Where("LOWER(name) LIKE ? ESCAPE '\\' OR LOWER(nickname) LIKE ? ESCAPE '\\'", pattern, pattern)
	}
	if len(roles) > 0 {
		db = db.Where("role IN ?", roles)
	}
	if len(statuses) > 0 {
		db = db.Where("status IN ?", statuses)
	}
	if len(bannedFilters) == 1 {
		if bannedFilters[0] {
			db = db.Where("banned_timestamp > ?", utils.GetLocalTime())
		} else {
			db = db.Where("banned_timestamp IS NULL OR banned_timestamp <= ?", utils.GetLocalTime())
		}
	}
	for _, clause := range adminUserSortClauses(request.Sort) {
		db = db.Order(clause)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		klog.Errorf("failed to count users: %v", err)
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "count users failed"))
		return
	}
	users := make([]*model.User, 0, request.PageSize)
	if err := db.Select("id, name, role, status, extra_balance, attributes, banned_timestamp, ban_restrictions").
		Offset((request.Page - 1) * request.PageSize).
		Limit(request.PageSize).
		Find(&users).Error; err != nil {
		klog.Errorf("failed to list users: %v", err)
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "list users failed"))
		return
	}

	items := make([]UserResp, 0, len(users))
	for _, user := range users {
		items = append(items, mgr.buildUserResponse(user, c))
	}
	resputil.Success(c, resputil.NewPage(items, total, request.Page, request.PageSize))
}

func (mgr *UserMgr) buildUserResponse(user *model.User, c *gin.Context) UserResp {
	banned := service.IsUserBanned(user.BannedTimestamp)
	extraBalance := 0.0
	if mgr.isBillingFeatureEnabled(c) {
		extraBalance = service.ToDisplayPoints(user.ExtraBalance)
	}
	return UserResp{
		ID:              user.ID,
		Name:            user.Name,
		Role:            user.Role,
		Status:          user.Status,
		ExtraBalance:    extraBalance,
		Attributes:      user.Attributes,
		Banned:          banned,
		PermanentBanned: service.IsUserPermanentlyBanned(user.BannedTimestamp),
		BannedTimestamp: user.BannedTimestamp,
		BanRestrictions: service.EffectiveUserBanRestrictions(banned, user.BanRestrictions.Data()),
	}
}
