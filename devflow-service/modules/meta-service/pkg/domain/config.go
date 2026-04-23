package domain

type LogConfig struct {
	Level  string `mapstructure:"level" json:"level" yaml:"level"`
	Format string `mapstructure:"format" json:"format" yaml:"format"`
}

type ServerConfig struct {
	Port int `mapstructure:"port" json:"port" yaml:"port"`
}

type PostgresConfig struct {
	DSN                    string `mapstructure:"dsn" json:"dsn" yaml:"dsn"`
	MaxOpenConns           int    `mapstructure:"max_open_conns" json:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns           int    `mapstructure:"max_idle_conns" json:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetimeMinutes int    `mapstructure:"conn_max_lifetime_minutes" json:"conn_max_lifetime_minutes" yaml:"conn_max_lifetime_minutes"`
}

type OtelConfig struct {
	Endpoint    string `mapstructure:"endpoint" json:"endpoint" yaml:"endpoint"`
	ServiceName string `mapstructure:"service_name" json:"service_name" yaml:"service_name"`
}
