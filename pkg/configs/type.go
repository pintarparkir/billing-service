package configs

// Config holds billing-service configuration.
// All fields are loaded from environment variables with safe defaults.
type Config struct {
	AppName  string `env:"APP_NAME" envDefault:"billing-service"`
	AppEnv   string `env:"APP_ENV" envDefault:"local"`
	GrpcPort string `env:"GRPC_PORT" envDefault:"9091"` // gRPC port (s2s) — billing has no REST surface

	DbHost     string `env:"DB_HOST" envDefault:"localhost"`
	DbPort     string `env:"DB_PORT" envDefault:"5432"`
	DbUsername string `env:"DB_USERNAME" envDefault:"postgres"`
	DbPassword string `env:"DB_PASSWORD" envDefault:"postgres"`
	DbName     string `env:"DB_NAME" envDefault:"billing_service"`
	DbMaxOpen  int    `env:"DB_MAX_OPEN" envDefault:"25"`
	DbMaxIdle  int    `env:"DB_MAX_IDLE" envDefault:"10"`

	RabbitURL      string `env:"RABBIT_URL" envDefault:"amqp://guest:guest@localhost:5672/"`
	RabbitExchange string `env:"RABBIT_EXCHANGE" envDefault:"parkirpintar.events"`
	RabbitQueue    string `env:"RABBIT_QUEUE" envDefault:"billing.events"`

	OTLPEndpoint string `env:"OTLP_ENDPOINT" envDefault:"localhost:4317"`

	// ── Pricing tariffs (IDR) ──
	BookingFeeIDR    int64 `env:"BOOKING_FEE_IDR" envDefault:"5000"`
	HourlyRateIDR    int64 `env:"HOURLY_RATE_IDR" envDefault:"5000"`
	OvernightFlatIDR int64 `env:"OVERNIGHT_FLAT_IDR" envDefault:"20000"`
	CancelFeeIDR     int64 `env:"CANCEL_FEE_IDR" envDefault:"5000"`
	NoShowFeeIDR     int64 `env:"NO_SHOW_FEE_IDR" envDefault:"5000"`
	CancelGraceMin   int   `env:"CANCEL_GRACE_MINUTES" envDefault:"2"`
}

// ConfigLoader controls the source of config (env file path, etc.).
type ConfigLoader struct {
	Env     string
	EnvFile string
}
