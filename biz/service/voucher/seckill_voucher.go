package voucher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
	"xzdp/biz/dal/mysql"
	"xzdp/biz/dal/redis"
	voucherModel "xzdp/biz/model/voucher"

	// "xzdp/biz/pkg/constants"
	"xzdp/biz/utils"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/streadway/amqp"
)

type SeckillVoucherService struct {
	RequestContext *app.RequestContext
	Context        context.Context
}

// 初始化RabbitMQ连接
func InitRabbitMQ() (*amqp.Connection, error) {
    conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
    if err != nil {
        return nil, fmt.Errorf("连接RabbitMQ失败: %v", err)
    }
    return conn, nil
}

// 初始化生产者
func InitProducer(conn *amqp.Connection) (*amqp.Channel, error) {
    ch, err := conn.Channel()
    if err != nil {
        return nil, fmt.Errorf("创建Channel失败: %v", err)
    }
    
    // 声明持久化队列和交换机
    _, err = ch.QueueDeclare(
        "seckill_queue",  // 队列名
        true,             // 持久化
        false,            // 自动删除
        false,            // 排他性
        false,            // 不等待
        nil,
    )
    if err != nil {
        return nil, fmt.Errorf("声明队列失败: %v", err)
    }

    return ch, nil
}

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
	// 2. 执行Lua脚本校验库存和用户资格
    user := utils.GetUser(h.Context)
    luaScript := `
        local voucherId = KEYS[1]
        local userId = ARGV[1]
        local stockKey = "seckill:stock:" .. voucherId
        local orderKey = "seckill:order:" .. voucherId

        if tonumber(redis.call('GET', stockKey)) <= 0 then
            return 1
        end
        if redis.call('SISMEMBER', orderKey, userId) == 1 then
            return 2
        end
        redis.call('DECR', stockKey)
        redis.call('SADD', orderKey, userId)
        return 0
    `
    
	
	result, err := redis.RedisClient.Eval(
        h.Context,
        luaScript,
        []string{strconv.FormatInt(*req, 10)}, // KEYS
        user.ID,                               // ARGV
    ).Int()
    if err != nil {
        return nil, fmt.Errorf("Lua脚本执行失败: %v", err)
    }

    switch result {
    case 1:
        return nil, errors.New("库存不足")
    case 2:
        return nil, errors.New("请勿重复下单")
    }
	

	// 3. 发送消息到RabbitMQ
    conn, err := InitRabbitMQ()
    ch, _ := InitProducer(conn)
    if err != nil {
        return nil, fmt.Errorf("初始化RabbitMQ失败: %v", err)
    }

    orderMsg := map[string]interface{}{
        "voucher_id": *req,
        "user_id":    user.ID,
    }
    msgBody, _ := json.Marshal(orderMsg)

    err = ch.Publish(
        "",              // 默认交换机
        "seckill_queue", // 队列名
        false,           // 强制标志
        false,           // 立即标志
        amqp.Publishing{
            DeliveryMode: amqp.Persistent, // 持久化消息
            ContentType:  "application/json",
            Body:         msgBody,
        },
    )
    if err != nil {
        return nil, fmt.Errorf("消息发送失败: %v", err)
    }
    hlog.Info("消息发送成功")

    return nil, nil // 返回成功，实际订单异步处理
}


func (h *SeckillVoucherService) createOrder(voucherId int64, userId int64) (resp *int64, err error) {
	//3.判断是否已经购买
	// userId := utils.GetUser(h.Context).GetID()
	// err = mysql.QueryVoucherOrderByVoucherID(h.Context, userId, voucherId)
	// if err != nil {
	// 	return nil, err
	// }
	//4.扣减库存
	err = mysql.UpdateVoucherStock(h.Context, voucherId)
	if err != nil {
		return nil, err
	}
	//5.创建订单
	orderId, err := redis.NextId(h.Context, "order")
	if err != nil {
		return nil, err
	}
	voucherOrder := &voucherModel.VoucherOrder{
		ID:         orderId,
		UserId:     userId,
		VoucherId:  voucherId,
		PayTime:    time.Now().Format("2006-01-02T15:04:05+08:00"),
		UseTime:    time.Now().Format("2006-01-02T15:04:05+08:00"),
		RefundTime: time.Now().Format("2006-01-02T15:04:05+08:00"),
	}
	
	err = mysql.CreateVoucherOrder(h.Context, voucherOrder)
	if err != nil {
		return nil, err
	}
	return &orderId, nil
}



// 消费者逻辑==================================================================================
type SeckillConsumer struct {
    service   *SeckillVoucherService
    conn      *amqp.Connection  // 保留 Connection 而非 Channel
    queueName string
    workerNum int               // 消费者协程数量
}

func NewSeckillConsumer(service *SeckillVoucherService, conn *amqp.Connection, workerNum int) *SeckillConsumer {
    return &SeckillConsumer{
        service:   service,
        conn:      conn,    // 多个消费者复用这个连接
        queueName: "seckill_queue",
        workerNum: workerNum,  // 通过参数指定消费者数量
    }
}

func (c *SeckillConsumer) Start() {
    for i := 0; i < c.workerNum; i++ {
        go c.startWorker(i + 1)
    }
}

func (c *SeckillConsumer) startWorker(workerID int) {
    // 每个 worker 创建独立的 Channel
    ch, err := c.conn.Channel()
    if err != nil {
        hlog.Fatalf("Worker %d 创建 Channel 失败: %v", workerID, err)
    }
    defer ch.Close()

    // 设置 QoS（预取计数），避免单个 worker 过载
    err = ch.Qos(
        1,     // 每个 worker 同时处理的最大消息数
        0,     // 预取大小（0 表示不限制）
        false, // 仅对当前 Channel 生效
    )
    if err != nil {
        hlog.Fatalf("Worker %d 设置 QoS 失败: %v", workerID, err)
    }

    // 声明队列（确保队列存在）
    // 即使多个 Worker 调用 QueueDeclare，只要队列名相同，RabbitMQ 会确保队列只创建一次​（幂等性）。
    _, err = ch.QueueDeclare(
        c.queueName,
        true,  // 持久化队列
        false, // 非自动删除
        false, // 非排他
        false, // 不等待
        nil,
    )
    if err != nil {
        hlog.Fatalf("Worker %d 声明队列失败: %v", workerID, err)
    }

    // 消费消息
    msgs, err := ch.Consume(
        c.queueName,
        "",              // 消费者标签
        false,           // 关闭自动 ACK
        false,           // 非排他
        false,           // 不等待
        false,           // 无额外参数
        nil,
    )
    if err != nil {
        hlog.Fatalf("Worker %d 注册消费者失败: %v", workerID, err)
    }

    hlog.Infof("Worker %d 已启动", workerID)
    for msg := range msgs {
        c.processMessage(workerID, msg)
    }
}

// 处理单条消息
func (c *SeckillConsumer) processMessage(workerID int, msg amqp.Delivery) {
    var data struct {
        VoucherID int64 `json:"voucher_id"`
        UserID    int64 `json:"user_id"`
    }
    if err := json.Unmarshal(msg.Body, &data); err != nil {
        hlog.Errorf("Worker %d 消息解析失败: %v", workerID, err)
        msg.Nack(false, false) // 丢弃无效消息
        return
    }

    // 调用订单创建逻辑
    orderId, err := c.service.createOrder(data.VoucherID, data.UserID)
    if err != nil {
        hlog.Errorf("Worker %d 创建订单失败: %v", workerID, err)
        msg.Nack(false, true) // 重新入队重试
        return
    }

    hlog.Infof("Worker %d 订单创建成功: %d", workerID, *orderId)
    msg.Ack(false)
}