package voucher

import (
	"context"
	"time"

	"xzdp/biz/dal/mysql"
	voucher "xzdp/biz/model/voucher"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type CreateSeckillVoucherService struct {
	RequestContext *app.RequestContext
	Context        context.Context
}

func NewCreateSeckillVoucherService(Context context.Context, RequestContext *app.RequestContext) *CreateSeckillVoucherService {
	return &CreateSeckillVoucherService{RequestContext: RequestContext, Context: Context}
}

func (h *CreateSeckillVoucherService) Run(req *voucher.SeckillVoucher) (resp *int64, err error) {
	//defer func() {
	// hlog.CtxInfof(h.Context, "req = %+v", req)
	// hlog.CtxInfof(h.Context, "resp = %+v", resp)
	//}()
	// todo edit your code
	hlog.Info("Create SeckillVoucher")
	// Step 1: 创建 Voucher 数据并插入 tb_voucher 表
	newVoucher := &voucher.Voucher{
		ShopId:      req.ShopId,	
		Title:       req.Title,
		SubTitle:    req.SubTitle,
		Rules:       req.Rules,
		PayValue:    req.PayValue,
		ActualValue: req.ActualValue,
		Type:        req.Type,
		Status:      1, // 默认状态为 1 (未使用/有效)
		CreateTime:  time.Now().Format("2006-01-02 15:04:05"),
		UpdateTime:  time.Now().Format("2006-01-02 15:04:05"),
	}

	// 插入数据到 tb_voucher 表
	mysql.DB.Create(&newVoucher)

	// 获取新插入的 Voucher ID
	voucherId := newVoucher.ID

	// Step 2: 创建 SeckillVoucher 数据并插入 tb_seckill_voucher 表
	seckillVoucher := &voucher.SeckillVoucher{
		VoucherId: voucherId,
		Stock:     req.Stock,
		BeginTime: req.BeginTime,
		EndTime:   req.EndTime,
	}

	// 插入数据到 tb_seckill_voucher 表
	mysql.DB.Select("voucher_id", "stock", "begin_time", "end_time").Create(seckillVoucher)

	// 返回新创建的 voucherId
	return &voucherId, nil
}
