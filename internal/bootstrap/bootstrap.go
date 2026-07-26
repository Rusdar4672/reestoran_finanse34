package bootstrap

import (
	"github.com/gin-gonic/gin"
	"github.com/yourusername/restaurant-finance/internal/adapter"
	"github.com/yourusername/restaurant-finance/internal/api"
	"github.com/yourusername/restaurant-finance/internal/config"
	"github.com/yourusername/restaurant-finance/internal/repository"
	"github.com/yourusername/restaurant-finance/internal/service"
)

type Application struct {
	Router *gin.Engine
	Store  *repository.Store
}

type Services struct {
	Store   *repository.Store
	Finance *service.FinanceService
	Excel   *service.ExcelService
	POS     *service.POSService
}

func (a *Application) Close() error {
	sqlDB, err := a.Store.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func New(cfg *config.Config, allowedOrigin string) (*Application, error) {
	gin.SetMode(gin.ReleaseMode)
	services, err := NewServices(cfg)
	if err != nil {
		return nil, err
	}
	return &Application{
		Router: api.NewRouter(services.Finance, services.Excel, services.POS, allowedOrigin),
		Store:  services.Store,
	}, nil
}

func NewServices(cfg *config.Config) (*Services, error) {
	store, err := repository.ConnectDB(cfg)
	if err != nil {
		return nil, err
	}
	finance := service.NewFinanceService(store)
	registry := adapter.NewRegistry(
		adapter.NewHTTPPOSAdapter("rkeeper"),
		adapter.NewHTTPPOSAdapter("iiko"),
		adapter.NewHTTPPOSAdapter("generic"),
	)
	return &Services{
		Store:   store,
		Finance: finance,
		Excel:   service.NewExcelService(finance),
		POS:     service.NewPOSService(finance, registry),
	}, nil
}
