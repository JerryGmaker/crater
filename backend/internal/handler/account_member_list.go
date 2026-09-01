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
	"github.com/raids-lab/crater/internal/util"
)

type accountMemberPageQuery struct {
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=10" binding:"min=1,max=200"`
	Search   string `form:"search"`
	Sort     string `form:"sort"`
}

var accountMemberSortFields = map[string]string{
	"name":       "users.name",
	"role":       "user_accounts.role",
	"accessmode": "user_accounts.access_mode",
}

func validateAccountMemberSort(sortValue string) error {
	parts := strings.Split(sortValue, ",")
	if len(parts) > 3 {
		return bizerr.BadRequest.ParameterError.New("sort accepts at most 3 fields")
	}
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		name := strings.TrimPrefix(part, "-")
		if _, ok := accountMemberSortFields[name]; !ok {
			return bizerr.BadRequest.ParameterError.New(fmt.Sprintf("unsupported sort field %q", name))
		}
		if _, ok := seen[name]; ok {
			return bizerr.BadRequest.ParameterError.New(fmt.Sprintf("duplicate sort field %q", name))
		}
		seen[name] = struct{}{}
	}
	return nil
}

func accountMemberSortClauses(sortValue string) []string {
	clauses := make([]string, 0, 4)
	for _, part := range strings.Split(sortValue, ",") {
		name := strings.TrimPrefix(part, "-")
		direction := "ASC"
		if strings.HasPrefix(part, "-") {
			direction = "DESC"
		}
		clauses = append(clauses, accountMemberSortFields[name]+" "+direction)
	}
	return append(clauses, "users.id ASC")
}

func parseAccountMemberEnumFilters(c *gin.Context, key string, allowed map[uint]struct{}) ([]uint, error) {
	values := c.QueryArray(key)
	parsed := make([]uint, 0, len(values))
	seen := make(map[uint]struct{}, len(values))
	for _, raw := range values {
		value, err := strconv.ParseUint(raw, 10, 8)
		if err != nil {
			return nil, bizerr.BadRequest.ParameterError.New(fmt.Sprintf("unsupported %s %q", key, raw))
		}
		numeric := uint(value)
		if _, ok := allowed[numeric]; !ok {
			return nil, bizerr.BadRequest.ParameterError.New(fmt.Sprintf("unsupported %s %q", key, raw))
		}
		if _, ok := seen[numeric]; ok {
			continue
		}
		seen[numeric] = struct{}{}
		parsed = append(parsed, numeric)
	}
	return parsed, nil
}

func bindAccountMemberPageQuery(c *gin.Context) (accountMemberPageQuery, []uint, []uint, error) {
	var request accountMemberPageQuery
	if err := c.ShouldBindQuery(&request); err != nil {
		return accountMemberPageQuery{}, nil, nil, bizerr.BadRequest.ParameterError.Wrap(
			err,
			"invalid account member list query",
		)
	}
	request.Search = strings.TrimSpace(request.Search)
	if utf8.RuneCountInString(request.Search) > 128 {
		return accountMemberPageQuery{}, nil, nil, bizerr.BadRequest.ParameterError.New(
			"search accepts at most 128 characters",
		)
	}
	if request.Page > int(^uint(0)>>1)/request.PageSize {
		return accountMemberPageQuery{}, nil, nil, bizerr.BadRequest.ParameterError.New(
			"page is too large for page_size",
		)
	}
	if request.Sort == "" {
		request.Sort = "name"
	}
	if err := validateAccountMemberSort(request.Sort); err != nil {
		return accountMemberPageQuery{}, nil, nil, err
	}

	roles, err := parseAccountMemberEnumFilters(c, "role", map[uint]struct{}{
		uint(model.RoleUser):  {},
		uint(model.RoleAdmin): {},
	})
	if err != nil {
		return accountMemberPageQuery{}, nil, nil, err
	}
	accessModes, err := parseAccountMemberEnumFilters(c, "accessmode", map[uint]struct{}{
		uint(model.AccessModeRO): {},
		uint(model.AccessModeRW): {},
	})
	if err != nil {
		return accountMemberPageQuery{}, nil, nil, err
	}
	return request, roles, accessModes, nil
}

func bindBillingMemberUserIDs(c *gin.Context) ([]uint, error) {
	rawIDs := c.QueryArray("user_id")
	if len(rawIDs) > 200 {
		return nil, bizerr.BadRequest.ParameterError.New("user_id accepts at most 200 values")
	}
	userIDs := make([]uint, 0, len(rawIDs))
	seen := make(map[uint]struct{}, len(rawIDs))
	for _, raw := range rawIDs {
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || value == 0 {
			return nil, bizerr.BadRequest.ParameterError.New(fmt.Sprintf("invalid user_id %q", raw))
		}
		userID := uint(value)
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}

func (mgr *AccountMgr) listAccountMembersPage(c *gin.Context, accountID uint) {
	request, roles, accessModes, err := bindAccountMemberPageQuery(c)
	if err != nil {
		resputil.HandleError(c, err)
		return
	}

	db := query.GetDB().WithContext(c).
		Table("users").
		Joins("JOIN user_accounts ON user_accounts.user_id = users.id").
		Where("users.deleted_at IS NULL").
		Where("user_accounts.deleted_at IS NULL").
		Where("user_accounts.account_id = ?", accountID)
	if request.Search != "" {
		pattern := util.ContainsPattern(request.Search)
		db = db.Where(
			"(LOWER(users.name) LIKE ? ESCAPE '\\' OR LOWER(users.nickname) LIKE ? ESCAPE '\\')",
			pattern,
			pattern,
		)
	}
	if len(roles) > 0 {
		db = db.Where("user_accounts.role IN ?", roles)
	}
	if len(accessModes) > 0 {
		db = db.Where("user_accounts.access_mode IN ?", accessModes)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		klog.Errorf("failed to count account members: %v", err)
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "count account members failed"))
		return
	}
	for _, clause := range accountMemberSortClauses(request.Sort) {
		db = db.Order(clause)
	}

	items := make([]UserProjectGetResp, 0, request.PageSize)
	if err := db.Select(
		"users.id, users.name, user_accounts.role, user_accounts.access_mode, users.attributes, user_accounts.quota",
	).
		Offset((request.Page - 1) * request.PageSize).
		Limit(request.PageSize).
		Scan(&items).Error; err != nil {
		klog.Errorf("failed to list account members: %v", err)
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "list account members failed"))
		return
	}

	resputil.Success(c, resputil.NewPage(items, total, request.Page, request.PageSize))
}

// UserListAccountMembersPage returns account members using the shared page protocol.
//
//	@Summary	分页获取账户成员
//	@Tags		Project
//	@Produce	json
//	@Security	Bearer
//	@Param		aid		path	uint	true	"Account ID"
//	@Param		page		query	int		false	"Page number"
//	@Param		page_size	query	int		false	"Page size, 1-200"
//	@Param		search		query	string	false	"Search username or nickname"
//	@Param		role		query	int		false	"Role filter; repeatable"
//	@Param		accessmode	query	int		false	"Access mode filter; repeatable"
//	@Param		sort		query	string	false	"Sort fields"
//	@Success	200			{object}	resputil.Response[resputil.Page[UserProjectGetResp]]
//	@Router		/v1/accounts/{aid}/users/page [get]
func (mgr *AccountMgr) UserListAccountMembersPage(c *gin.Context) {
	var req ProjectGetReq
	if err := c.ShouldBindUri(&req); err != nil {
		resputil.HandleError(c, bizerr.BadRequest.InvalidRequest.Wrap(err, "invalid request"))
		return
	}
	token := util.GetToken(c)
	if err := mgr.checkUserInAccount(c, token.UserID, req.ID); err != nil {
		resputil.HandleError(c, err)
		return
	}
	if _, err := mgr.validateAccount(c, req.ID); err != nil {
		resputil.HandleError(c, err)
		return
	}
	mgr.listAccountMembersPage(c, req.ID)
}

// AdminListAccountMembersPage returns account members for platform administrators.
//
//	@Summary	管理员分页获取账户成员
//	@Tags		Project
//	@Produce	json
//	@Security	Bearer
//	@Param		aid		path	uint	true	"Account ID"
//	@Param		page		query	int		false	"Page number"
//	@Param		page_size	query	int		false	"Page size, 1-200"
//	@Param		search		query	string	false	"Search username or nickname"
//	@Param		role		query	int		false	"Role filter; repeatable"
//	@Param		accessmode	query	int		false	"Access mode filter; repeatable"
//	@Param		sort		query	string	false	"Sort fields"
//	@Success	200			{object}	resputil.Response[resputil.Page[UserProjectGetResp]]
//	@Router		/v1/admin/accounts/userIn/{aid}/page [get]
func (mgr *AccountMgr) AdminListAccountMembersPage(c *gin.Context) {
	var req ProjectGetReq
	if err := c.ShouldBindUri(&req); err != nil {
		resputil.HandleError(c, bizerr.BadRequest.InvalidRequest.Wrap(err, "invalid request"))
		return
	}
	if _, err := mgr.validateAccount(c, req.ID); err != nil {
		resputil.HandleError(c, err)
		return
	}
	mgr.listAccountMembersPage(c, req.ID)
}
