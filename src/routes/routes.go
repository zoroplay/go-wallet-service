package routes

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	pbWallet "github.com/zoroplay/go-wallet-service/grpc/protobuf"
	"github.com/zoroplay/go-wallet-service/initializers"
	"google.golang.org/grpc"
)

// router and DB instance
type App struct {
	E  *echo.Echo
	DB *sql.DB
	pbWallet.UnimplementedWalletServiceServer
}

//var defaultConfigPath = "/gaming/application/config/config.ini"

// Initialize initializes the app with predefined configuration
func (a *App) Initialize() {

	a.DB = initializers.DbInstance()

	// init webserver
	a.E = echo.New()
	a.E.Use(middleware.Gzip())
	//a.E.IPExtractor = echo.ExtractIPFromXFFHeader()
	a.E.IPExtractor = echo.ExtractIPFromRealIPHeader()

	// add recovery middleware to make the system null safe
	a.E.Use(middleware.Recover())

	// setup log format and parameters to log for every request
	a.E.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "time=${time_rfc3339}, method=${method}, uri=${uri}, status=${status}, latency=${latency_human}, ip=${remote_ip} \n",
		Output: log.Writer(),
	}))

	//setup CORS
	a.E.Use(middleware.CORSWithConfig(middleware.CORSConfig{

		AllowOrigins: []string{"*"}, // in production limit this to only known hosts
		AllowHeaders: []string{"*"},
		//AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderXForwardedFor,echo.HeaderXRealIP,echo.HeaderAuthorization},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))

	a.setRouters()

}

// setRouters sets the all required router
func (a *App) setRouters() {

	a.E.POST("/status", a.Status)
	a.E.GET("/status", a.Status)
}

// Run the app on it's router
func (a *App) Run() {

	server := fmt.Sprintf("%s:%s", os.Getenv("wallet_system_host"), os.Getenv("wallet_system_port"))
	log.Printf(" HTTP listening on %s ", server)
	a.E.Logger.Fatal(a.E.Start(server))
}

func (a *App) GRPC() {

	server := fmt.Sprintf("%s:%s", os.Getenv("wallet_system_host"), os.Getenv("wallet_system_grpc_port"))

	lis, err := net.Listen("tcp", server)
	if err != nil {

		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pbWallet.RegisterWalletServiceServer(s, a)
	log.Printf("GRPC server listening at %v", lis.Addr())

	if err := s.Serve(lis); err != nil {

		log.Fatalf("failed to serve: %v", err)
	}
}
