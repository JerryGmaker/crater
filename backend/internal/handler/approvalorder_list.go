package handler

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"k8s.io/klog/v2"

	"github.com/raids-lab/crater/dao/model"
	"github.com/raids-lab/crater/dao/query"
	"github.com/raids-lab/crater/internal/bizerr"
	"github.com/raids-lab/crater/internal/resputil"
	"github.com/raids-lab/crater/internal/util"
)

const approvalOrderMaxSearchRunes = 128

type approvalOrderListQuery struct {
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=10" binding:"min=1,max=200"`
	Search   string `form:"search"`
	Sort     string `form:"sort"`
}

func bindApprovalOrderListQuery(c *gin.Context) (approvalOrderListQuery, []string, []string, error) {
	var request approvalOrderListQuery
	if err := c.ShouldBindQuery(&request); err != nil {
		return approvalOrderListQuery{}, nil, nil, bizerr.BadRequest.ParameterError.Wrap(
			err,
			"invalid approval order list query",
		)
	}
	request.Search = strings.TrimSpace(request.Search)
	if utf8.RuneCountInString(request.Search) > approvalOrderMaxSearchRunes {
		return approvalOrderListQuery{}, nil, nil, bizerr.BadRequest.ParameterError.New(
			"search accepts at most 128 characters",
		)
	}
	if request.Page > int(^uint(0)>>1)/request.PageSize {
		return approvalOrderListQuery{}, nil, nil, bizerr.BadRequest.ParameterError.New(
			"page is too large for page_size",
		)
	}

	types, err := validateApprovalOrderFilters(c.QueryArray("type"), approvalOrderTypes)
	if err != nil {
		return approvalOrderListQuery{}, nil, nil, err
	}
	statuses, err := validateApprovalOrderFilters(c.QueryArray("status"), approvalOrderStatuses)
	if err != nil {
		return approvalOrderListQuery{}, nil, nil, err
	}
	if request.Sort == "" {
		request.Sort = "-createdAt"
	}
	if err := validateApprovalOrderSort(request.Sort); err != nil {
		return approvalOrderListQuery{}, nil, nil, err
	}
	return request, types, statuses, nil
}

var approvalOrderTypes = map[string]struct{}{
	string(model.ApprovalOrderTypeJob):     {},
	string(model.ApprovalOrderTypeDataset): {},
}

var approvalOrderStatuses = map[string]struct{}{
	string(model.ApprovalOrderStatusPending):   {},
	string(model.ApprovalOrderStatusApproved):  {},
	string(model.ApprovalOrderStatusRejected):  {},
	string(model.ApprovalOrderStatusCancelled): {},
}

func validateApprovalOrderFilters(values []string, allowed map[string]struct{}) ([]string, error) {
	valid := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return nil, bizerr.BadRequest.ParameterError.New(
				fmt.Sprintf("unsupported approval order filter %q", value),
			)
		}
		valid = append(valid, value)
	}
	return valid, nil
}

var approvalOrderSortFields = map[string]string{
	"name":      "name",
	"type":      "type",
	"status":    "status",
	"createdAt": "created_at",
}

func validateApprovalOrderSort(sortValue string) error {
	parts := strings.Split(sortValue, ",")
	if len(parts) > 3 {
		return bizerr.BadRequest.ParameterError.New("sort accepts at most 3 fields")
	}
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		name := strings.TrimPrefix(part, "-")
		if _, ok := approvalOrderSortFields[name]; !ok {
			return bizerr.BadRequest.ParameterError.New(fmt.Sprintf("unsupported sort field %q", name))
		}
		if _, ok := seen[name]; ok {
			return bizerr.BadRequest.ParameterError.New(fmt.Sprintf("duplicate sort field %q", name))
		}
		seen[name] = struct{}{}
	}
	return nil
}

func approvalOrderSortClauses(sortValue string) []string {
	clauses := make([]string, 0, 4)
	for _, part := range strings.Split(sortValue, ",") {
		name := strings.TrimPrefix(part, "-")
		direction := "ASC"
		if strings.HasPrefix(part, "-") {
			direction = "DESC"
		}
		clauses = append(clauses, approvalOrderSortFields[name]+" "+direction)
	}
	clauses = append(clauses, "id DESC")
	return clauses
}

func (mgr *ApprovalOrderMgr) listApprovalOrdersPage(c *gin.Context, creatorID *uint) {
	request, types, statuses, err := bindApprovalOrderListQuery(c)
	if err != nil {
		resputil.HandleError(c, err)
		return
	}

	db := query.ApprovalOrder.WithContext(c).UnderlyingDB().Model(&model.ApprovalOrder{})
	if creatorID != nil {
		db = db.Where("creator_id = ?", *creatorID)
	}
	if len(types) > 0 {
		db = db.Where("type IN ?", types)
	}
	if len(statuses) > 0 {
		db = db.Where("status IN ?", statuses)
	}
	if request.Search != "" {
		db = db.Where("LOWER(name) LIKE ? ESCAPE '\\'", util.ContainsPattern(request.Search))
	}
	for _, clause := range approvalOrderSortClauses(request.Sort) {
		db = db.Order(clause)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		klog.Errorf("failed to count approval orders: %v", err)
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "count approval orders failed"))
		return
	}

	orders := make([]*model.ApprovalOrder, 0)
	if err := db.Preload("Creator").Preload("Reviewer").
		Offset((request.Page - 1) * request.PageSize).
		Limit(request.PageSize).
		Find(&orders).Error; err != nil {
		klog.Errorf("failed to list approval orders: %v", err)
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "list approval orders failed"))
		return
	}

	resputil.Success(c, resputil.NewPage(
		convertToApprovalOrderResps(orders),
		total,
		request.Page,
		request.PageSize,
	))
}

// ListMyApprovalOrdersPage returns the current user's approval orders in a page.
//
//	@Summary	获取我的审批工单分页
//	@Description	按统一分页协议获取当前用户创建的审批工单
//	@Tags		approvalorder
//	@Produce	json
//	@Security	Bearer
//	@Param		page		query	int	false	"Page number"
//	@Param		page_size	query	int	false	"Page size, 1-200"
//	@Param		search		query	string	false	"Search order names"
//	@Param		type		query	string	false	"Filter by order type; repeatable"
//	@Param		status		query	string	false	"Filter by order status; repeatable"
//	@Param		sort		query	string	false	"Sort fields"
//	@Success	200	{object}	resputil.Response[resputil.Page[ApprovalOrderResp]]
//	@Router		/v1/approvalorder/page [get]
func (mgr *ApprovalOrderMgr) ListMyApprovalOrdersPage(c *gin.Context) {
	token := util.GetToken(c)
	if token.UserID == 0 {
		resputil.Error(c, "cannot get user id", resputil.NotSpecified)
		return
	}
	mgr.listApprovalOrdersPage(c, &token.UserID)
}

// ListAllApprovalOrdersPage returns all approval orders in a page for administrators.
//
//	@Summary	管理员获取审批工单分页
//	@Description	按统一分页协议获取所有审批工单
//	@Tags		approvalorder
//	@Produce	json
//	@Security	Bearer
//	@Param		page		query	int	false	"Page number"
//	@Param		page_size	query	int	false	"Page size, 1-200"
//	@Param		search		query	string	false	"Search order names"
//	@Param		type		query	string	false	"Filter by order type; repeatable"
//	@Param		status		query	string	false	"Filter by order status; repeatable"
//	@Param		sort		query	string	false	"Sort fields"
//	@Success	200	{object}	resputil.Response[resputil.Page[ApprovalOrderResp]]
//	@Router		/v1/admin/approvalorder/page [get]
func (mgr *ApprovalOrderMgr) ListAllApprovalOrdersPage(c *gin.Context) {
	mgr.listApprovalOrdersPage(c, nil)
}

type ApprovalOrderSummaryResp struct {
	TotalPending    int64  `json:"totalPending"`
	PendingJobDelay int64  `json:"pendingJobDelay"`
	PendingDataset  int64  `json:"pendingDataset"`
	UpdatedAt       string `json:"updatedAt"`
}

// GetApprovalOrderSummary returns aggregate counts independent of the current page.
//
//	@Summary	获取审批工单聚合统计
//	@Description	返回与列表分页无关的审批工单统计数据
//	@Tags		approvalorder
//	@Produce	json
//	@Security	Bearer
//	@Success	200	{object}	resputil.Response[ApprovalOrderSummaryResp]
//	@Router		/v1/admin/approvalorder/summary [get]
func (mgr *ApprovalOrderMgr) GetApprovalOrderSummary(c *gin.Context) {
	db := query.ApprovalOrder.WithContext(c).UnderlyingDB().Model(&model.ApprovalOrder{})
	count := func(conditions ...any) (int64, error) {
		var total int64
		err := db.Session(&gorm.Session{}).Where(conditions[0], conditions[1:]...).Count(&total).Error
		return total, err
	}

	totalPending, err := count("status = ?", string(model.ApprovalOrderStatusPending))
	if err != nil {
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "count pending approval orders failed"))
		return
	}
	pendingJobDelay, err := count(
		"status = ? AND type = ? AND COALESCE((content->>'approvalorderExtensionHours')::int, 0) > 0",
		string(model.ApprovalOrderStatusPending),
		string(model.ApprovalOrderTypeJob),
	)
	if err != nil {
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "count pending job approval orders failed"))
		return
	}
	pendingDataset, err := count(
		"status = ? AND type = ?",
		string(model.ApprovalOrderStatusPending),
		string(model.ApprovalOrderTypeDataset),
	)
	if err != nil {
		resputil.HandleError(c, bizerr.Internal.DatabaseError.Wrap(err, "count pending dataset approval orders failed"))
		return
	}

	result := ApprovalOrderSummaryResp{
		TotalPending:    totalPending,
		PendingJobDelay: pendingJobDelay,
		PendingDataset:  pendingDataset,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	resputil.Success(c, result)
}
