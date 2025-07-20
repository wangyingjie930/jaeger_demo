package main

import (
	"nexus/internal/pkg/bootstrap"
	"nexus/internal/pkg/logger"
	"nexus/internal/service/promotion/application"
	"nexus/internal/service/promotion/infrastructure"
	"nexus/internal/service/promotion/interfaces"

	_ "github.com/go-sql-driver/mysql" // 导入mysql驱动
	"go.opentelemetry.io/otel"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	serviceName = "promotion-service"
)

func main() {
	// 初始化 tracer, nacos 等通用组件
	bootstrap.Init()

	bootstrap.StartService(bootstrap.AppInfo{
		ServiceName: serviceName,
		Port:        8087,
		RegisterHandlers: func(appCtx bootstrap.AppCtx) {
			// 1. **连接数据库 (基础设施)**
			// dsn := bootstrap.GetCurrentConfig().DB.Source
			dsn := bootstrap.GetCurrentConfig().Infra.Mysql.Addrs // 应从配置获取
			db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
			if err != nil {
				logger.Logger.Error().Err(err).Msgf("failed to connect to database with gorm: %v", err)
			}

			// 2. **自动迁移 (基础设施)**
			// 使用在 infrastructure 包中定义的 GORM 模型
			err = db.AutoMigrate(&infrastructure.CouponTemplateModel{}, &infrastructure.UserCouponModel{})
			if err != nil {
				logger.Logger.Error().Err(err).Msgf("WARN: failed to auto migrate gorm models: %v", err)
			}

			// 3. **创建仓储实例 (基础设施)**
			couponRepository := infrastructure.NewGormCouponRepository(db)

			// 4. **创建应用服务实例 (应用层)**
			// 将仓储接口注入到应用服务中
			tracer := otel.Tracer(serviceName)
			promoService := application.NewPromotionService(couponRepository, tracer)

			// 5. **创建HTTP处理器 (接口层)**
			// 将应用服务注入到HTTP处理器中
			promoHandler := interfaces.NewPromotionHandler(promoService)

			// 6. **启动服务并注册路由**
			promoHandler.RegisterRoutes(appCtx.Mux)

			logger.Logger.Printf("✅ Promotion service routes registered.")
		},
	})
}
