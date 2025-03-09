package dependencyinjection

import (
	
	"sync"
	"routing-service/pkg/config"
	"routing-service/pkg/database"
	"routing-service/pkg/logger"
	"routing-service/routers"
	"routing-service/routers/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

func LoadDependencies() {
	fx.New(
		fx.Invoke(initializeConnectionsAndConfig),
		routerModule,
		commonModule,
		configModule,
		cacheModule,
		// kafkaFactory,
		AuthModule,

		ReqResInfoModule,
		
		ResponseHandlerModule,
		loggerModule,
	
		externalAPI,
		DeviceSecurityModule,
		EncryptionServiceModule,

		fx.Invoke(routers.RegisterRoutes),
		fx.Invoke(registerRouterGroup),
		fx.Invoke(registerCommonRoutes),
		fx.Invoke(registerCommandExecuteRoutes),
		fx.Invoke(registerAuthRoutes),
		fx.Invoke(registerPushDataRoutes),
		fx.Invoke(registerDeviceSecurityRoutes),

		fx.Invoke(StartKafkaConsumers),
		fx.Invoke(registerDeviceManagementRoutes),

		fx.Invoke(serveHttpRequests), // This should be invoked in the end always.
	)
}

func StartKafkaConsumers(
	logger logger.ILogger,
	cfg *config.Configuration,
	cmdExecutionUpdatesHandler cmdExecUpdatesServiceIntf.ICommandExecutionUpdates,
	responseHandler responseHandler.ITAPPushHandler,
	dlmsPushHandler responseHandler.IDLMSPushHandler,
	dlmsResponseHandler cmdExecUpdatesServiceIntf.IDLMSResponseHandler,
	inactiveMeterHandler inactiveMeterHandlerServiceIntf.IUtilityService,
	scheduledCmdExecRequestHandler scheduledCmdServiceInt.IScheduledCommandConsumerService,
	mdmsCmdExecRequestHandler mdmsCmdServiceIntf.IMDMSCmdExecConsumerService,
	satCmdExecRequestHandler satCmdServiceIntf.ISATCmdExecConsumerService,
	encryptionService encryptionServiceInterface.IEncryptionService,
) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		cmdExecutionUpdatesHandler.ConsumeCommandExecUpdatesKafkaMessages()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		responseHandler.ConsumeDecodedPushPackets()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		inactiveMeterHandler.ConsumeInactiveMeterMessages()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		dlmsPushHandler.ConsumeDlmsPushFromResponseKafka()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		dlmsResponseHandler.StartConsumingDLMSResponses()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		scheduledCmdExecRequestHandler.ScheduledCommandExecRequestKafkaConsumer()
	}()

	if cfg.APIClientConfig.MDMS_CLIENT_NAME == "POLARIS" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mdmsCmdExecRequestHandler.MDMSCmdExecRequestKafkaConsumer()
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		satCmdExecRequestHandler.SATCmdExecRequestKafkaConsumer()
	}()
}

var v1RouterGroup *gin.RouterGroup

func registerRouterGroup(
	r *routers.Routes,
	cfg *config.Configuration,
	authService authService.IAuthService,
	authController *authControllers.AuthController,
	commonService commonService.ICommonService,
) {
	v1RouterGroup = r.Router.Group(
		"/v1",
		middleware.RequestResponseInterceptor(),
		middleware.JWTTokenValidation(cfg),
		middleware.SetRequestContextMiddleware(authService, commonService),
		middleware.ValidateIdTokenMiddleware(cfg, authService),
		middleware.RouteAccessMiddleware(authService),
		middleware.ValidateRequestParamsMiddleware(),
	)

	r.Router.POST("/v1/auth/token", middleware.RequestResponseInterceptor(), authController.GetToken) // !TODO: this needs to be removed from here

}

func registerAuthRoutes(r *routers.Routes, c *authControllers.AuthController) {
	r.AddAuthRoutes(v1RouterGroup, c)
}

func registerCommonRoutes(r *routers.Routes, commonController *commonControllers.CommonController) {
	r.AddCommonRoutes(v1RouterGroup, commonController)
}

func serveHttpRequests(r *routers.Routes) {
	logger.GetLogger().Fatalf("%v", r.Router.Run(config.ServerConfig()))
}

func registerCommandExecuteRoutes(
	r *routers.Routes,
	c *commandExecuteController.CommandExecuteController,
	authService authService.IAuthService,
	commandInfoService commandExecService.ICommandInfoService,
	commonService commonService.ICommonService,
	commandExecInfoService commandExecService.ICommandExecutionInfoService,
	deviceInfoService dvcManagement.IDeviceInfoService,
	cfg *config.Configuration) {
	r.AddCommandExecutionRoutes(v1RouterGroup, c, authService, commandInfoService, commonService, commandExecInfoService, deviceInfoService, cfg)
}

func registerDeviceManagementRoutes(r *routers.Routes, c *DeviceManagementController.DeviceManagementController) {
	r.AddDeviceManagementRoutes(v1RouterGroup, c)
}

func registerPushDataRoutes(r *routers.Routes, c *PushdataController.PushDataController) {
	r.AddPushDataRoutes(v1RouterGroup, c)
}

func registerDeviceSecurityRoutes(r *routers.Routes, c *DeviceSecurityController.DeviceSecurityController) {
	r.AddDeviceSecurityRoutes(v1RouterGroup, c)
}

func initializeConnectionsAndConfig(cacheConnectionService cacheService.ICacheConnection, logger logger.ILogger) {
	if _, err := config.SetupConfig(); err != nil {
		logger.Fatalf("config SetupConfig() error: %s", err)
	}

	err := database.SetupDbConnection(logger)
	if err != nil {
		logger.Fatalf("erorr while setting DB connection in <initializeConnectionsAndConfig>:%v", err)
	}
	cacheConnectionService.GetRedisClient(nil)
}
