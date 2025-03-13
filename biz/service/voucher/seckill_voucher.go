package voucher

import (
	"context"
	"errors"
	"fmt"
	// "strconv"
	"sync"
	"time"
	"xzdp/biz/dal/mysql"
	"xzdp/biz/dal/redis"
	voucherModel "xzdp/biz/model/voucher"
	"xzdp/biz/pkg/constants"

	// "xzdp/biz/pkg/constants"
	"xzdp/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/gorm"
)

type SeckillVoucherService struct {
	RequestContext *app.RequestContext
	Context        context.Context
}

var (
	wg sync.WaitGroup
)

// 全局锁池：每个 userId 对应一个独立的锁
var userLocks sync.Map // key: userId (string), value: *sync.Mutex

func NewSeckillVoucherService(Context context.Context, RequestContext *app.RequestContext) *SeckillVoucherService {
	return &SeckillVoucherService{RequestContext: RequestContext, Context: Context}
}

func (h *SeckillVoucherService) Run(req *int64) (resp *int64, err error) {
	//defer func() {
	// hlog.CtxInfof(h.Context, "req = %+v", req)
	// hlog.CtxInfof(h.Context, "resp = %+v", resp)
	//}()
	// todo edit your code
	//0.查询秒杀券
	voucher, err := mysql.QueryVoucherByID(h.Context, *req)
	if err != nil {
		return nil, err
	}
	fmt.Println(voucher)
	now := time.Now()
	//1.判断是否开始&&结束
	layout := "2006-01-02T15:04:05+08:00"
	beginTime, _ := time.Parse(layout, voucher.GetBeginTime())
	endTime, _ := time.Parse(layout, voucher.GetEndTime())
	if beginTime.After(now) {
		return nil, errors.New("秒杀还未开始")
	}
	if endTime.Before(now) {
		return nil, errors.New("秒杀已经结束")
	}
	//2.判断库存是否充足
	if voucher.GetStock() <= 0 {
		return nil, errors.New("已抢空")
	}
	user := utils.GetUser(h.Context)
	fmt.Println(user)
	//===========================================================================================================
	mutex := redis.RedsyncClient.NewMutex(fmt.Sprintf("%s:%s", constants.VOUCHER_LOCK_KEY, user.NickName))
	err = mutex.Lock()
	if err != nil {
		return nil, errors.New("获取分布式锁失败")
	}
	// 使用通道来接收 createOrder 的结果
	resultCh := make(chan *int64, 1)
	errCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		order, err := h.createOrder(*req)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- order
		}
	}()
	if ok, err := mutex.Unlock(); !ok || err != nil {
		return nil, err
	}
	select {
	case order := <-resultCh:
		return order, nil
	case err := <-errCh:
		return nil, err
	case <-h.Context.Done():
		return nil, errors.New("请求超时")
	}
	//===================================================================================

	// userLock := getUserLock(*req)
    // userLock.Lock()
    // defer userLock.Unlock()
    
    // order, err := h.createOrder(*req)
    // if err != nil {
    //     return nil, err
    // }
    // return order, nil
}

// func getUserLock(userId int64) *sync.Mutex {
//     key := strconv.FormatInt(userId, 10)
//     lock, _ := userLocks.LoadOrStore(key, &sync.Mutex{})
//     return lock.(*sync.Mutex)
// }

func (h *SeckillVoucherService) createOrder(voucherId int64) (resp *int64, err error) {
	// //3.判断是否已经购买
	// userId := utils.GetUser(h.Context).GetID()
	// err = mysql.QueryVoucherOrderByVoucherID(h.Context, userId, voucherId)
	// if err != nil {
	// 	return nil, err
	// }
	// //4.扣减库存
	// err = mysql.UpdateVoucherStock(h.Context, voucherId)
	// if err != nil {
	// 	return nil, err
	// }
	// //5.创建订单
	// orderId, err := redis.NextId(h.Context, "order")
	// if err != nil {
	// 	return nil, err
	// }
	// voucherOrder := &voucherModel.VoucherOrder{
	// 	ID:         orderId,
	// 	UserId:     userId,
	// 	VoucherId:  voucherId,
	// 	PayTime:    time.Now().Format("2006-01-02T15:04:05+08:00"),
	// 	UseTime:    time.Now().Format("2006-01-02T15:04:05+08:00"),
	// 	RefundTime: time.Now().Format("2006-01-02T15:04:05+08:00"),
	// }
	
	// err = mysql.CreateVoucherOrder(h.Context, voucherOrder)
	// if err != nil {
	// 	return nil, err
	// }
	// return &orderId, nil

	// ===============================================================================
	// 这个事务的正确性还没有进行验证
	userId := utils.GetUser(h.Context).GetID()

	// 2. 生成订单ID（Redis原子操作，非事务部分）
	orderId, err := redis.NextId(h.Context, "order")
	if err != nil {
		return nil, err
	}
	
	err = mysql.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 扣减库存（传入事务对象）
        if err := mysql.UpdateVoucherStock(h.Context, tx, voucherId); err != nil {
            if errors.Is(err, gorm.ErrRecordNotFound) {
                return fmt.Errorf("库存不足")
            }
            return fmt.Errorf("扣减库存失败: %w", err)
        }

        

        // 3. 创建订单对象
        voucherOrder := &voucherModel.VoucherOrder{
            ID:         orderId,
            UserId:     userId,
            VoucherId:  voucherId,
            PayTime:    time.Now().Format(time.RFC3339),
            UseTime:    time.Now().Format(time.RFC3339),
            RefundTime: time.Now().Format(time.RFC3339),
        }

        // 4. 插入订单（传入事务对象）
        if err := mysql.CreateVoucherOrder(h.Context, tx, voucherOrder); err != nil {
            return fmt.Errorf("创建订单失败: %w", err)
        }

        return nil
	})

	if err != nil {
		return nil, err
	}
	return &orderId, nil
	// ====================================================================================
}
